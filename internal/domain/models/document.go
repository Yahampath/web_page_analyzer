package models

import (
	"bytes"
	"context"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// Document represents a parsed HTML document and provides methods for analysis.
type Document struct {
	htmlNode *html.Node
	baseURL  *url.URL
	body     []byte
}

// NewDocument creates a new Document from a byte slice and a base URL.
func NewDocument(body []byte, baseURL *url.URL) (*Document, error) {
	node, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return &Document{
		htmlNode: node,
		baseURL:  baseURL,
		body:     body,
	}, nil
}

// HTMLVersion detects the HTML version of the document.
func (d *Document) HTMLVersion(ctx context.Context) string {
	tokenizer := html.NewTokenizer(bytes.NewReader(d.body))
	var doctype string
loop:
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.DoctypeToken:
			tokens := tokenizer.Token()
			doctype = tokens.String()
			break loop
		case html.ErrorToken:
			break loop
		}
	}
	doctypeLower := strings.ToLower(doctype)
	switch {
	case strings.Contains(doctypeLower, "html 5") || strings.TrimSpace(doctypeLower) == "<!doctype html>":
		return "HTML5"
	case strings.Contains(doctypeLower, "html 4.01 strict"):
		return "HTML 4.01 Strict"
	case strings.Contains(doctypeLower, "html 4.01 transitional"):
		return "HTML 4.01 Transitional"
	case strings.Contains(doctypeLower, "xhtml 1.0 strict"):
		return "XHTML 1.0 Strict"
	case strings.Contains(doctypeLower, "xhtml 1.0 transitional"):
		return "XHTML 1.0 Transitional"
	default:
		return doctype
	}
}

// Title extracts the title of the document.
func (d *Document) Title(ctx context.Context) string {
	titleNode := findNode(d.htmlNode, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "title"
	})

	if titleNode != nil && titleNode.FirstChild != nil {
		return titleNode.FirstChild.Data
	}
	return ""
}

// Headings counts the occurrences of each heading type (h1-h6).
func (d *Document) Headings(ctx context.Context) map[string]int {
	counts := make(map[string]int)
	headingNodes := findAllNodes(d.htmlNode, func(node *html.Node) bool {
		if node.Type == html.ElementNode {
			switch node.Data {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				return true
			}
		}
		return false
	})

	for _, node := range headingNodes {
		counts[node.Data]++
	}
	return counts
}

// Links counts the number of internal and external links.
func (d *Document) Links(ctx context.Context) (internal int, external int) {
	links := d.collectLinks(ctx)
	for _, link := range links {
		if link.isInternal {
			internal++
		} else {
			external++
		}
	}
	return internal, external
}

// CollectLinks gathers all links from the document.
func (d *Document) CollectLinks(ctx context.Context) []map[string]interface{} {
	var collectedLinks []map[string]interface{}
	links := d.collectLinks(ctx)
	for _, link := range links {
		collectedLinks = append(collectedLinks, map[string]interface{}{
			"url":        link.url,
			"isInternal": link.isInternal,
		})
	}
	return collectedLinks
}

// HasLoginForm checks if the document contains a login form.
func (d *Document) HasLoginForm(ctx context.Context) bool {
	formNode := findNode(d.htmlNode, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "form" && formHasPassword(ctx, n)
	})
	return formNode != nil
}

// --- Private Helper Functions ---

type linkInfo struct {
	url        string
	isInternal bool
}

func (d *Document) collectLinks(ctx context.Context) []linkInfo {
	var links []linkInfo
	linkNodes := findAllNodes(d.htmlNode, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "a"
	})

	for _, node := range linkNodes {
		href := getHref(ctx, node)
		if href == "" {
			continue
		}
		absoluteURL, err := d.baseURL.Parse(href)
		if err != nil {
			continue
		}
		if absoluteURL.Scheme != "http" && absoluteURL.Scheme != "https" {
			continue
		}
		isInternal := getCanonicalHost(ctx, absoluteURL) == getCanonicalHost(ctx, d.baseURL)
		links = append(links, linkInfo{url: absoluteURL.String(), isInternal: isInternal})
	}
	return links
}

func formHasPassword(ctx context.Context, form *html.Node) bool {
	passwordInputNode := findNode(form, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "input" {
			for _, attr := range n.Attr {
				if attr.Key == "type" && attr.Val == "password" {
					return true
				}
			}
		}
		return false
	})
	return passwordInputNode != nil
}

func getHref(ctx context.Context, n *html.Node) string {
	for _, attr := range n.Attr {
		if attr.Key == "href" {
			return attr.Val
		}
	}
	return ""
}

func getCanonicalHost(ctx context.Context, u *url.URL) string {
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		return host
	}
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		return host
	}
	return host + ":" + port
}

func traverse(n *html.Node, visitor func(n *html.Node) (keepGoing bool)) {
	var f func(*html.Node)
	f = func(n *html.Node) {
		if !visitor(n) {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
}

func findNode(n *html.Node, predicate func(n *html.Node) bool) *html.Node {
	var result *html.Node
	traverse(n, func(node *html.Node) bool {
		if predicate(node) {
			result = node
			return false // stop traversal
		}
		return true // continue traversal
	})
	return result
}

func findAllNodes(n *html.Node, predicate func(n *html.Node) bool) []*html.Node {
	var results []*html.Node
	traverse(n, func(node *html.Node) bool {
		if predicate(node) {
			results = append(results, node)
		}
		return true // continue traversal
	})
	return results
}
