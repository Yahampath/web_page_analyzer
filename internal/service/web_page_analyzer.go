package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"web_page_analyzer/internal/domain/adaptors"
	"web_page_analyzer/internal/domain/models"
	"web_page_analyzer/internal/pkg/errors"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
	"golang.org/x/sync/errgroup"
)

type WebPageAnalyzer interface {
	Analyze(url string) (string, error)
}

type linkInfo struct {
	url        string
	isInternal bool
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

func (a *Analyzer) Analyze(ctx context.Context, userURL string) (models.AnalysisResult, error) {
	a.log.Debug(`analyze web page started...`)
	result := models.AnalysisResult{}

	// Create error group with context
	g, ctx := errgroup.WithContext(ctx)

	baseURLChan := make(chan *url.URL, 1)
	bodyChan := make(chan []byte, 1)
	docChan := make(chan *html.Node, 1)

	g.Go(func() error {
		baseURL, err := url.Parse(userURL)
		if err != nil {
			a.log.WithError(err).Error(`failed to parse url`)
			return errors.Wrap(err, `failed to parse url`)
		}
		baseURLChan <- baseURL
		return nil
	})

	g.Go(func() error {
		bodyByte, respCode, err := a.getWebPage(ctx, userURL)
		if err != nil {
			result.StatusCode = respCode
			a.log.WithError(err).Error(`failed to get web page`)
			return errors.Wrap(err, `failed to get web page`)
		}
		bodyChan <- bodyByte

		doc, err := html.Parse(bytes.NewReader(bodyByte))
		if err != nil {
			a.log.WithError(err).Error(`failed to parse html`)
			return errors.Wrap(err, `failed to parse html`)
		}
		docChan <- doc
		return nil
	})

	// Wait for both parallel operations
	if err := g.Wait(); err != nil {
		return result, err
	}

	// Get results from channels
	baseURL := <-baseURLChan
	bodyByte := <-bodyChan
	doc := <-docChan

	// HTML Version
	HTMLVersion, err := a.getHTMLVersion(ctx, bodyByte)
	if err != nil {
		a.log.WithError(err).Error(`failed to get html version`)
		return result, errors.Wrap(err, `failed to get html version`)
	}

	// Title
	title, err := a.getTitle(ctx, doc)
	if err != nil {
		a.log.WithError(err).Error(`failed to get title`)
		return result, errors.Wrap(err, `failed to get title`)
	}

	// Headings
	headings, err := a.countHeadings(ctx, doc)
	if err != nil {
		a.log.WithError(err).Error(`failed to count headings`)
		return result, errors.Wrap(err, `failed to count headings`)
	}

	// Links
	internalLinks, externalLinks, err := a.countLinks(ctx, doc, baseURL)
	if err != nil {
		a.log.WithError(err).Error(`failed to count links`)
		return result, errors.Wrap(err, `failed to count links`)
	}

	// Inaccessible links
	links, err := a.collectLinks(ctx, doc, baseURL)
	if err != nil {
		a.log.WithError(err).Error(`failed to collect links`)
		return result, errors.Wrap(err, `failed to collect links`)
	}

	inaccessible, err := a.checkLinksAccessibility(ctx, links)
	if err != nil {
		a.log.WithError(err).Error(`failed to check links accessibility`)
		return result, errors.Wrap(err, `failed to check links accessibility`)
	}

	// Login form
	hasForm, err := a.hasLoginForm(ctx, doc)
	if err != nil {
		a.log.WithError(err).Error(`failed to check login form`)
		return result, errors.Wrap(err, `failed to check login form`)
	}

	result.HTMLVersion = HTMLVersion
	result.Title = title
	result.Headings = headings
	result.InternalLinks = internalLinks
	result.ExternalLinks = externalLinks
	result.InaccessibleLinks = inaccessible
	result.HasLoginForm = hasForm

	a.log.Debug(`analyze web page ended...`)
	return result, nil
}

func (a *Analyzer) getWebPage(ctx context.Context, userURL string) ([]byte, int, error) {
	bodyByte, responseCode, err := a.webClient.Do(ctx, userURL, http.MethodGet)
	if err != nil {
		a.log.WithError(err).Error(`url is invalid`)
		return nil, 400, err
	}

	if responseCode != http.StatusOK {
		a.log.Errorf(`url is invalid. status code: %v`, responseCode)
		return nil, responseCode, errors.New(fmt.Sprintf(`url is invalid states code is %d`, responseCode))
	}

	return bodyByte, responseCode, nil
}

func (a *Analyzer) getHTMLVersion(_ context.Context, body []byte) (string, error) {
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
		return "HTML 4.01 Strict", nil
	case strings.Contains(doctypeLower, "html 4.01 transitional"):
		return "HTML 4.01 Transitional", nil
	case strings.Contains(doctypeLower, "xhtml 1.0 strict"):
		return "XHTML 1.0 Strict", nil
	case strings.Contains(doctypeLower, "xhtml 1.0 transitional"):
		return "XHTML 1.0 Transitional", nil
	case strings.Contains(doctypeLower, "html 5") || strings.TrimSpace(doctypeLower) == "<!doctype html>":
		return "HTML5", nil
	default:
		return doctype, nil
	}
}

func (a *Analyzer) getTitle(_ context.Context, n *html.Node) (string, error) {
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
	return title, nil
}

func (a *Analyzer) countHeadings(_ context.Context, n *html.Node) (map[string]int, error) {
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
	return counts, nil
}

func (a *Analyzer) countLinks(ctx context.Context, doc *html.Node, baseURL *url.URL) (int, int, error) {
	links, err := a.collectLinks(ctx, doc, baseURL)
	if err != nil {
		return 0, 0, errors.Wrap(err, `failed to collect links`)
	}
	internal, external := 0, 0
	for _, link := range links {
		if link.isInternal {
			internal++
		} else {
			external++
		}
	}
	return internal, external, nil
}

func (a *Analyzer) collectLinks(ctx context.Context, doc *html.Node, baseURL *url.URL) ([]linkInfo, error) {
	var links []linkInfo
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			href := a.getHref(ctx, n)
			if href == "" {
				return
			}
			absoluteURL, err := baseURL.Parse(href)
			if err != nil {
				a.log.WithError(err).Error(`failed to parse url: `, href)
				return
			}
			if absoluteURL.Scheme != "http" && absoluteURL.Scheme != "https" {
				return
			}
			isInternal := a.getCanonicalHost(ctx, absoluteURL) == a.getCanonicalHost(ctx, baseURL)
			links = append(links, linkInfo{url: absoluteURL.String(), isInternal: isInternal})
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)
	return links, nil
}

func (a *Analyzer) getHref(_ context.Context, n *html.Node) string {
	for _, attr := range n.Attr {
		if attr.Key == "href" {
			return attr.Val
		}
	}
	return ""
}

func (a *Analyzer) getCanonicalHost(_ context.Context, u *url.URL) string {
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

func (a *Analyzer) checkLinksAccessibility(ctx context.Context, links []linkInfo) (int, error) {
	var wg sync.WaitGroup
	results := make(chan bool, len(links))
	sem := make(chan struct{}, 10) // Concurrency limiter

	for _, link := range links {
		wg.Add(1)
		go func(ctx context.Context, url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			_, responseCode, err := a.webClient.Do(ctx, url, http.MethodHead)
			if err != nil {
				results <- false
				return
			}

			if responseCode >= 400 {
				results <- false
			} else {
				results <- true
			}
		}(ctx, link.url)
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
	return inaccessible, nil
}

func (a *Analyzer) hasLoginForm(ctx context.Context, doc *html.Node) (bool, error) {
	var hasLogin bool
	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "form" {
			if a.formHasPassword(ctx, n) {
				hasLogin = true
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)
	return hasLogin, nil
}

func (a *Analyzer) formHasPassword(_ context.Context, form *html.Node) bool {
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
