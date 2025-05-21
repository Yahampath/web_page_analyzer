package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"web_page_analyzer/internal/domain/adaptors"
	"web_page_analyzer/internal/domain/models"
	"web_page_analyzer/internal/pkg/errors"
	"web_page_analyzer/internal/pkg/worker_pool"

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

	initialTasks := []struct {
		id   string
		task func(ctx context.Context) (any, error)
	}{
		{"parseUrl", a.parseUrlTask(userURL)},
		{"getWebPage", a.getWebPageTask(userURL)},
	}
	initialPool := worker_pool.NewWorkerPool(ctx, 2, true, a.log)
	err := a.RunTasksWithWorkerPool(ctx, initialTasks, initialPool)
	if err != nil {
		a.log.WithContext(ctx).WithError(err).Error(`failed during initial analysis tasks`)
		return a.result, err
	}


	pool := worker_pool.NewWorkerPool(ctx, runtime.NumCPU(), true, a.log)
	tasks := []struct {
		id   string
		task func(ctx context.Context) (any, error)
	}{
		{"getHTMLVersion", a.getHTMLVersionTask()},
		{"getTitle", a.getTitleTask()},
		{"countHeadings", a.countHeadingsTask()},
		{"countLinks", a.countLinksTask()},
		{"checkLinksAccessibility", a.checkLinksAccessibilityTask(pool)},
		{"hasLoginForm", a.hasLoginFormTask()},
	}
	err = a.RunTasksWithWorkerPool(ctx, tasks, pool)
	if err != nil {
		a.log.WithContext(ctx).WithError(err).Error(`failed during analysis tasks`)
		return a.result, err
	}

	a.log.Debug(`analyze web page ended...`)
	return a.result, nil
}

func (a *Analyzer) RunTasksWithWorkerPool(ctx context.Context, tasks []struct {
	id   string
	task func(ctx context.Context) (any, error)
}, pool *worker_pool.WorkerPool) error {

	for _, t := range tasks {
		pool.Submit(t.id, t.task)
	}

	for {
		select {
		case res, ok := <-pool.ResultsCh:
			if !ok {
				pool.ResultsCh = nil
				continue
			}
			if res.Err != nil {
				return res.Err
			}
			switch res.ID {
			case "parseUrl":
				if parsedUrl, ok := res.Result.(*url.URL); ok {
					a.result.Mux.Lock()
					a.result.BaseUrl = parsedUrl
					a.result.Mux.Unlock()
				}
			case "getWebPage":
			case "getHTMLVersion":
				if htmlVersion, ok := res.Result.(string); ok {
					a.result.Mux.Lock()
					a.result.HTMLVersion = htmlVersion
					a.result.Mux.Unlock()
				}
			case "getTitle":
				if title, ok := res.Result.(string); ok {
					a.result.Mux.Lock()
					a.result.Title = title
					a.result.Mux.Unlock()
				}
			case "countHeadings":
				if counts, ok := res.Result.(map[string]int); ok {
					a.result.Mux.Lock()
					a.result.Headings = counts
					a.result.Mux.Unlock()
				}
			case "countLinks":
				if counts, ok := res.Result.(map[string]int); ok {
					a.result.Mux.Lock()
					a.result.InternalLinks = counts["internal"]
					a.result.ExternalLinks = counts["external"]
					a.result.Mux.Unlock()
				}
			case "checkLinksAccessibility":
				if inaccessible, ok := res.Result.(int); ok {
					a.result.Mux.Lock()
					a.result.InaccessibleLinks = inaccessible
					a.result.Mux.Unlock()
				}
			case "hasLoginForm":
				if hasLogin, ok := res.Result.(bool); ok {
					a.result.Mux.Lock()
					a.result.HasLoginForm = hasLogin
					a.result.Mux.Unlock()
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
		if pool.ResultsCh == nil {
			break
		}
	}
	return nil
}

func (a *Analyzer) parseUrlTask(userURL string) func(ctx context.Context) (any, error) {
	return func(ctx context.Context) (any, error) {
		baseURL, err := url.Parse(userURL)
		if err != nil {
			a.log.WithContext(ctx).WithError(err).Error(`failed to parse url`)
			return nil, errors.Wrap(err, `failed to parse url`)
		}
		return baseURL, nil
	}
}

func (a *Analyzer) getWebPageTask(userURL string) func(ctx context.Context) (any, error) {
	return func(ctx context.Context) (any, error) {
		bodyByte, responseCode, err := a.webClient.Do(ctx, userURL, http.MethodGet)
		if err != nil {
			a.log.WithContext(ctx).WithError(err).Error(`url is invalid`)
			return nil, err
		}

		if responseCode != http.StatusOK {
			a.log.WithContext(ctx).Errorf(`url is invalid. status code: %v`, responseCode)
			return nil, errors.New(fmt.Sprintf(`url is invalid states code is %d`, responseCode))
		}

		doc, err := html.Parse(bytes.NewReader(bodyByte))
		if err != nil {
			a.log.WithContext(ctx).WithError(err).Error(`failed to parse html`)
			return nil, errors.Wrap(err, `failed to parse html`)
		}

		a.result.Mux.Lock()
		a.result.StatusCode = responseCode
		a.result.BodyByte = bodyByte
		a.result.HtmlNode = doc
		a.result.Mux.Unlock()
		return nil, nil
	}
}

func (a *Analyzer) getHTMLVersionTask() func(ctx context.Context) (any, error) {
	return func(ctx context.Context) (any, error) {
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
		htmlVersion := ``
		doctypeLower := strings.ToLower(doctype)
		switch {
		case strings.Contains(doctypeLower, "html 4.01 strict"):
			htmlVersion = "HTML 4.01 Strict"
		case strings.Contains(doctypeLower, "html 4.01 transitional"):
			htmlVersion = "HTML 4.01 Transitional"
		case strings.Contains(doctypeLower, "xhtml 1.0 strict"):
			htmlVersion = "XHTML 1.0 Strict"
		case strings.Contains(doctypeLower, "xhtml 1.0 transitional"):
			htmlVersion = "XHTML 1.0 Transitional"
		case strings.Contains(doctypeLower, "html 5") || strings.TrimSpace(doctypeLower) == "<!doctype html>":
			htmlVersion = "HTML5"
		default:
			htmlVersion = doctype
		}
		return htmlVersion, nil
	}
}

func (a *Analyzer) getTitleTask() func(ctx context.Context) (any, error) {
	return func(ctx context.Context) (any, error) {
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
		return title, nil
	}
}

func (a *Analyzer) countHeadingsTask() func(ctx context.Context) (any, error) {
	return func(ctx context.Context) (any, error) {
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
		return counts, nil
	}
}

func (a *Analyzer) countLinksTask() func(ctx context.Context) (any, error) {
	return func(ctx context.Context) (any, error) {
		links, err := a.collectLinks(ctx, a.result.HtmlNode, a.result.BaseUrl)
		if err != nil {
			return nil, errors.Wrap(err, `failed to collect links`)
		}
		internal, external := 0, 0
		for _, link := range links {
			if link.isInternal {
				internal++
			} else {
				external++
			}
		}
		return map[string]int{"internal": internal, "external": external}, nil
	}
}

func (a *Analyzer) checkLinksAccessibilityTask(pool *worker_pool.WorkerPool) func(ctx context.Context) (any, error) {
	return func(ctx context.Context) (any, error) {
		links, err := a.collectLinks(ctx, a.result.HtmlNode, a.result.BaseUrl)
		if err != nil {
			a.log.WithContext(ctx).WithError(err).Error(`failed to collect links`)
			return nil, errors.Wrap(err, `failed to collect links`)
		}

		for _, link := range links {
			linkURL := link.url
			pool.Submit("", func(ctx context.Context) (any, error) {
				_, responseCode, err := a.webClient.Do(ctx, linkURL, http.MethodHead)
				if err != nil {
					return false, nil
				}
				if responseCode >= 400 {
					return false, nil
				}
				return true, nil
			})
		}

		inaccessibleCount := 0
		for i := 0; i < len(links); i++ {
			res := <-pool.ResultsCh
			ok, _ := res.Result.(bool)
			if !ok {
				inaccessibleCount++
			}
		}
		return any(inaccessibleCount), nil
	}
}

func (a *Analyzer) hasLoginFormTask() func(ctx context.Context) (any, error) {
	return func(ctx context.Context) (any, error) {
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
		return hasLogin, nil
	}
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
