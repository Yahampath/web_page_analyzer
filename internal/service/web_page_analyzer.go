package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"web_page_analyzer/internal/domain/adaptors"
	"web_page_analyzer/internal/domain/models"
	"web_page_analyzer/internal/pkg/errors"

	"golang.org/x/sync/errgroup"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
)

type WebPageAnalyzer interface {
	Analyze(url string) (string, error)
}

type linkInfo struct {
	url        string
	isInternal bool
}

type webPageInfo struct {
	responseCode int
	bodyByte     []byte
	htmlNode     *html.Node
}

type Analyzer struct {
	log       *log.Logger
	webClient adaptors.WebClient
}

func NewAnalyzer(log *log.Logger, webClient adaptors.WebClient) *Analyzer {
	return &Analyzer{
		log:       log,
		webClient: webClient,
	}
}

func (a *Analyzer) Analyze(ctx context.Context, userURL string) (*models.AnalysisResult, error) {
	a.log.Debug(`analyze web page started...`)

	result := &models.AnalysisResult{}
	g, ctx := errgroup.WithContext(ctx)

	var (
		parsedURL *url.URL
		pageInfo  webPageInfo
	)

	g.Go(func() error {
		funcStartTime := time.Now()
		defer func() {
			a.log.Debugf("parseUrl took %v", time.Since(funcStartTime))
		}()
		u, err := parseUrl(ctx, userURL)
		if err != nil {
			a.log.WithContext(ctx).WithError(err).Error(`failed to parse url`)
			return err
		}
		parsedURL = u
		return nil
	})

	g.Go(func() error {
		funcStartTime := time.Now()
		defer func() {
			a.log.Debugf("getWebPage took %v", time.Since(funcStartTime))
		}()
		pi, err := getWebPage(ctx, userURL, a.webClient)
		if err != nil {
			a.log.WithContext(ctx).WithError(err).Error(`failed to get web page`)
			return err
		}
		pageInfo = pi
		return nil
	})

	if err := g.Wait(); err != nil {
		return result, errors.Wrap(err, "failed to prepare web page or URL")
	}

	result.BaseUrl = parsedURL
	result.StatusCode = pageInfo.responseCode
	result.BodyByte = pageInfo.bodyByte
	result.HtmlNode = pageInfo.htmlNode

	analyzeGroup, ctx := errgroup.WithContext(ctx)

	analyzeGroup.Go(func() error {
		funcStartTime := time.Now()
		defer func() {
			a.log.Debugf("checkLinksAccessibility took %v", time.Since(funcStartTime))
		}()
		links := collectLinks(ctx, result.HtmlNode, result.BaseUrl)
		inaccessibleLinks := checkLinksAccessibility(ctx, links)
		result.InaccessibleLinks = inaccessibleLinks
		return nil
	})

	analyzeGroup.Go(func() error {
		funcStartTime := time.Now()
		defer func() {
			a.log.Debugf("countLinks took %v", time.Since(funcStartTime))
		}()
		internal, external := countLinks(ctx, result.HtmlNode, result.BaseUrl)
		result.InternalLinks = internal
		result.ExternalLinks = external
		return nil
	})

	analyzeGroup.Go(func() error {
		funcStartTime := time.Now()
		defer func() {
			a.log.Debugf("countHeadings took %v", time.Since(funcStartTime))
		}()
		result.Headings = countHeadings(ctx, result.HtmlNode)
		return nil
	})

	analyzeGroup.Go(func() error {
		funcStartTime := time.Now()
		defer func() {
			a.log.Debugf("getTitle took %v", time.Since(funcStartTime))
		}()
		result.Title = getTitle(ctx, result.HtmlNode)
		return nil
	})

	analyzeGroup.Go(func() error {
		funcStartTime := time.Now()
		defer func() {
			a.log.Debugf("getHTMLVersion took %v", time.Since(funcStartTime))
		}()
		result.HTMLVersion = getHTMLVersion(ctx, result.BodyByte)
		return nil
	})

	analyzeGroup.Go(func() error {
		funcStartTime := time.Now()
		defer func() {
			a.log.Debugf("checkLoginForm took %v", time.Since(funcStartTime))
		}()
		result.HasLoginForm = hasLoginForm(ctx, result.HtmlNode)
		return nil
	})

	if err := analyzeGroup.Wait(); err != nil {
		return result, errors.Wrap(err, "failed to analyze web page")
	}

	a.log.Debug(`analyze web page ended...`)
	return result, nil
}

func parseUrl(ctx context.Context, userUrl string) (*url.URL, error) {
	baseURL, err := url.Parse(userUrl)
	if err != nil {
		return nil, err
	}

	if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
		return nil, errors.New("url is invalid")
	}

	return baseURL, nil
}

func getWebPage(ctx context.Context, userURL string, httpClient adaptors.WebClient) (webPageInfo, error) {
	var info webPageInfo
	bodyByte, responseCode, err := httpClient.Do(ctx, userURL, http.MethodGet)
	if err != nil {
		return info, err
	}

	if responseCode != http.StatusOK {
		return info, errors.New(fmt.Sprintf(`url is invalid states code is %d`, responseCode))
	}

	doc, err := html.Parse(bytes.NewReader(bodyByte))
	if err != nil {
		return info, err
	}

	info.bodyByte = bodyByte
	info.responseCode = responseCode
	info.htmlNode = doc

	return info, nil
}

func getHTMLVersion(ctx context.Context, body []byte) string {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
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
	case strings.Contains(doctypeLower, "html 4.01 strict"):
		return "HTML 4.01 Strict"
	case strings.Contains(doctypeLower, "html 4.01 transitional"):
		return "HTML 4.01 Transitional"
	case strings.Contains(doctypeLower, "xhtml 1.0 strict"):
		return "XHTML 1.0 Strict"
	case strings.Contains(doctypeLower, "xhtml 1.0 transitional"):
		return "XHTML 1.0 Transitional"
	case strings.Contains(doctypeLower, "html 5") || strings.TrimSpace(doctypeLower) == "<!doctype html>":
		return "HTML5"
	default:
		return doctype
	}
}

func getTitle(ctx context.Context, n *html.Node) string {
	var title string
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			title = n.FirstChild.Data
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(n)
	return title
}

func countHeadings(ctx context.Context, n *html.Node) map[string]int {
	counts := map[string]int{"h1": 0, "h2": 0, "h3": 0, "h4": 0, "h5": 0, "h6": 0}
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "h1":
				counts["h1"]++
			case "h2":
				counts["h2"]++
			case "h3":
				counts["h3"]++
			case "h4":
				counts["h4"]++
			case "h5":
				counts["h5"]++
			case "h6":
				counts["h6"]++
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(n)
	return counts
}

func countLinks(ctx context.Context, doc *html.Node, baseURL *url.URL) (int, int) {
	links := collectLinks(ctx, doc, baseURL)
	internal, external := 0, 0
	for _, link := range links {
		if link.isInternal {
			internal++
		} else {
			external++
		}
	}
	return internal, external
}

func collectLinks(ctx context.Context, doc *html.Node, baseURL *url.URL) []linkInfo {
	var links []linkInfo
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := getHref(ctx, n)
			if href == "" {
				return
			}
			absoluteURL, err := baseURL.Parse(href)
			if err != nil {
				return
			}
			if absoluteURL.Scheme != "http" && absoluteURL.Scheme != "https" {
				return
			}
			isInternal := getCanonicalHost(ctx, absoluteURL) == getCanonicalHost(ctx, baseURL)
			links = append(links, linkInfo{url: absoluteURL.String(), isInternal: isInternal})
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)
	return links
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

func checkLinksAccessibility(ctx context.Context, links []linkInfo) int {
	var wg sync.WaitGroup
	results := make(chan bool, len(links))
	sem := make(chan struct{}, 20)
	client := http.Client{Timeout: 1 * time.Second}
	defer client.CloseIdleConnections()

	for _, link := range links {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			resp, err := client.Head(url)
			if err != nil {
				results <- false
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 400 {
				results <- false
			} else {
				results <- true
			}
		}(link.url)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	inaccessible := 0
	for res := range results {
		if !res {
			inaccessible++
		}
	}
	return inaccessible
}

func hasLoginForm(ctx context.Context, doc *html.Node) bool {
	var hasLogin bool
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "form" {
			if formHasPassword(ctx, n) {
				hasLogin = true
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)
	return hasLogin
}

func formHasPassword(ctx context.Context, form *html.Node) bool {
	var hasPassword bool
	var traverseForm func(*html.Node)
	traverseForm = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" {
			for _, attr := range n.Attr {
				if attr.Key == "type" && attr.Val == "password" {
					hasPassword = true
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverseForm(c)
		}
	}
	traverseForm(form)
	return hasPassword
}
