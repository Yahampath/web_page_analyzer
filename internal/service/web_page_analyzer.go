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
	result    *models.AnalysisResult
}

func NewAnalyzer(log *log.Logger, webClient adaptors.WebClient) *Analyzer {
	return &Analyzer{
		log:       log,
		webClient: webClient,
		result:    &models.AnalysisResult{},
	}
}

func (a *Analyzer) Analyze(ctx context.Context, userURL string) (*models.AnalysisResult, error) {
	a.log.Debug(`analyze web page started...`)

	err := a.parseUrl(ctx, userURL)
	if err != nil {
		a.log.WithError(err).Error(`failed to parse url`)
		return nil, errors.Wrap(err, `failed to parse url`)
	}

	err = a.getWebPage(ctx, userURL)
	if err != nil {
		a.log.WithContext(ctx).WithError(err).Error(`failed to get web page`)
		return nil, errors.Wrap(err, `failed to get web page`)
	}

	// HTML Version
	 err = a.getHTMLVersion(ctx)
	if err != nil {
		a.log.WithContext(ctx).WithError(err).Error(`failed to get html version`)
		return a.result, errors.Wrap(err, `failed to get html version`)
	}

	// Title
	err = a.getTitle(ctx)
	if err != nil {
		a.log.WithContext(ctx).WithError(err).Error(`failed to get title`)
		return a.result, errors.Wrap(err, `failed to get title`)
	}

	// Headings
	err = a.countHeadings(ctx)
	if err != nil {
		a.log.WithContext(ctx).WithError(err).Error(`failed to count headings`)
		return a.result, errors.Wrap(err, `failed to count headings`)
	}

	// Links
	err = a.countLinks(ctx)
	if err != nil {
		a.log.WithContext(ctx).WithError(err).Error(`failed to count links`)
		return a.result, errors.Wrap(err, `failed to count links`)
	}

	err = a.checkLinksAccessibility(ctx)
	if err != nil {
		a.log.WithContext(ctx).WithError(err).Error(`failed to check links accessibility`)
		return a.result, errors.Wrap(err, `failed to check links accessibility`)
	}

	// Login form
	err = a.hasLoginForm(ctx)
	if err != nil {
		a.log.WithContext(ctx).WithError(err).Error(`failed to check login form`)
		return a.result, errors.Wrap(err, `failed to check login form`)
	}

	a.log.Debug(`analyze web page ended...`)
	return a.result, nil
}

func(a *Analyzer) parseUrl(ctx context.Context, userUrl string) error {
	baseURL, err := url.Parse(userUrl)
	if err != nil {
		a.log.WithContext(ctx).WithError(err).Error(`failed to parse url`)
		return errors.Wrap(err, `failed to parse url`)
	}
	a.result.Mux.Lock()
	defer a.result.Mux.Unlock()
	a.result.BaseUrl = baseURL
	return nil
}

func (a *Analyzer) getWebPage(ctx context.Context, userURL string)  error {
	bodyByte, responseCode, err := a.webClient.Do(ctx, userURL, http.MethodGet)
	if err != nil {
		a.log.WithContext(ctx).WithError(err).Error(`url is invalid`)
		return err
	}

	if responseCode != http.StatusOK {
		a.log.WithContext(ctx).Errorf(`url is invalid. status code: %v`, responseCode)
		return errors.New(fmt.Sprintf(`url is invalid states code is %d`, responseCode))
	}

	doc, err := html.Parse(bytes.NewReader(bodyByte))
	if err != nil {
		a.log.WithContext(ctx).WithError(err).Error(`failed to parse html`)
		return errors.Wrap(err, `failed to parse html`)
	}

	a.result.Mux.Lock()
	defer a.result.Mux.Unlock()
	a.result.StatusCode = responseCode
	a.result.BodyByte = bodyByte
	a.result.HtmlNode = doc
	return nil
}

func (a *Analyzer) getHTMLVersion(ctx context.Context) error {
	tokenizer := html.NewTokenizer(bytes.NewReader(a.result.BodyByte))
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
	htmlVersion  := ``
	doctypeLower := strings.ToLower(doctype)
	switch {
	case strings.Contains(doctypeLower, "html 4.01 strict"):
		htmlVersion = "HTML 4.01 Strict"
	case strings.Contains(doctypeLower, "html 4.01 transitional"):
		htmlVersion = "HTML 4.01 Transitional" 
	case strings.Contains(doctypeLower, "xhtml 1.0 strict"):
		htmlVersion =  "XHTML 1.0 Strict"
	case strings.Contains(doctypeLower, "xhtml 1.0 transitional"):
		htmlVersion =  "XHTML 1.0 Transitional"
	case strings.Contains(doctypeLower, "html 5") || strings.TrimSpace(doctypeLower) == "<!doctype html>":
		htmlVersion =  "HTML5"
	default:
		htmlVersion = doctype
	}

	a.result.Mux.Lock()
	defer a.result.Mux.Unlock()
	a.result.HTMLVersion = htmlVersion
	return nil
}

func (a *Analyzer) getTitle(ctx context.Context) error{
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
	traverse(a.result.HtmlNode)

	a.result.Mux.Lock()
	defer a.result.Mux.Unlock()
	a.result.Title = title
	return nil
}

func (a *Analyzer) countHeadings(ctx context.Context) error {
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
	traverse(a.result.HtmlNode)
	a.result.Mux.Lock()
	defer a.result.Mux.Unlock()
	a.result.Headings = counts
	return nil
}

func (a *Analyzer) countLinks(ctx context.Context) error {
	links, err := a.collectLinks(ctx, a.result.HtmlNode, a.result.BaseUrl)
	if err != nil {
		return errors.Wrap(err, `failed to collect links`)
	}
	internal, external := 0, 0
	for _, link := range links {
		if link.isInternal {
			internal++
		} else {
			external++
		}
	}

	a.result.Mux.Lock()
	defer a.result.Mux.Unlock()
	a.result.InternalLinks = internal
	a.result.ExternalLinks = external
	return nil
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

func (a *Analyzer) getHref(ctx context.Context, n *html.Node) string {
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

func (a *Analyzer) checkLinksAccessibility(ctx context.Context) (error) {
	links, err := a.collectLinks(ctx, a.result.HtmlNode, a.result.BaseUrl)
	if err != nil {
		a.log.WithContext(ctx).WithError(err).Error(`failed to collect links`)
		return errors.Wrap(err, `failed to collect links`)
	}

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

	a.result.Mux.Lock()
	defer a.result.Mux.Unlock()
	a.result.InaccessibleLinks = inaccessible
	return nil
}

func (a *Analyzer) hasLoginForm(ctx context.Context) error {
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
	traverse(a.result.HtmlNode)

	a.result.Mux.Lock()
	defer a.result.Mux.Unlock()
	a.result.HasLoginForm = hasLogin
	return nil
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
