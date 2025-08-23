package service

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"web_page_analyzer/internal/domain/adaptors"
	"web_page_analyzer/internal/domain/models"
	"web_page_analyzer/internal/pkg/contextkeys"
	"web_page_analyzer/internal/pkg/errors"

	log "github.com/sirupsen/logrus"
)

type WebPageAnalyzer interface {
	Analyze(url string) (string, error)
}

type webPageInfo struct {
	responseCode int
	bodyByte     []byte
}

type linkCheckerFunc func(ctx context.Context, links []map[string]interface{}) int

type Analyzer struct {
	log         *log.Logger
	webClient   adaptors.WebClient
	linkChecker linkCheckerFunc
}

func NewAnalyzer(log *log.Logger, webClient adaptors.WebClient) *Analyzer {
	return &Analyzer{
		log:         log,
		webClient:   webClient,
		linkChecker: checkLinksAccessibility,
	}
}

func (a *Analyzer) Analyze(ctx context.Context, userURL string) (*models.AnalysisResult, error) {
	reqID, _ := ctx.Value(contextkeys.CtxKeyRequestID{}).(string)
	log := a.log.WithFields(log.Fields{
		"request_id": reqID,
		"url":        userURL,
	})

	log.Info("starting web page analysis")

	// Phase 1: Prerequisites
	prereqPool := NewWorkerPool(10, func(ctx context.Context, payload interface{}) (interface{}, error) {
		task := payload.(func() (interface{}, error))
		return task()
	})
	prereqPool.Start(ctx)

	go func() {
		prereqPool.Submit(Job{ID: "1", Payload: func() (interface{}, error) {
			return parseUrl(ctx, userURL)
		}})
		prereqPool.Submit(Job{ID: "2", Payload: func() (interface{}, error) {
			return getWebPage(ctx, userURL, a.webClient)
		}})
		prereqPool.Close()
	}()

	var (
		parsedURL *url.URL
		pageInfo  webPageInfo
		prereqErr error
	)

	for res := range prereqPool.Results() {
		if res.Err != nil {
			prereqErr = res.Err
			break
		}
		switch res.JobID {
		case "1":
			parsedURL = res.Value.(*url.URL)
		case "2":
			pageInfo = res.Value.(webPageInfo)
		}
	}

	if prereqErr != nil {
		log.WithError(prereqErr).Error("failed to complete prerequisite tasks")
		return nil, prereqErr
	}

	// Phase 2: Main Analysis
	doc, err := models.NewDocument(pageInfo.bodyByte, parsedURL)
	if err != nil {
		log.WithError(err).Error("failed to create new document")
		return nil, errors.Wrap(err, "failed to create new document")
	}

	result := &models.AnalysisResult{
		BaseUrl:    parsedURL,
		StatusCode: pageInfo.responseCode,
	}

	if err := a.runAnalysisTasks(ctx, log, doc, result); err != nil {
		return nil, err
	}

	log.Info("finished web page analysis")
	return result, nil
}

func (a *Analyzer) runAnalysisTasks(ctx context.Context, log *log.Entry, doc *models.Document, result *models.AnalysisResult) error {
	// Use a worker pool for analysis tasks
	analysisPool := NewWorkerPool(6, func(ctx context.Context, payload interface{}) (interface{}, error) {
		task := payload.(func() (interface{}, error))
		return task()
	})
	analysisPool.Start(ctx)

	go func() {
		analysisPool.Submit(Job{ID: "1", Payload: func() (interface{}, error) {
			log.Debug("starting link accessibility check")
			links := doc.CollectLinks(ctx)
			inaccessibleLinks := a.linkChecker(ctx, links)
			log.Debug("finished link accessibility check")
			return inaccessibleLinks, nil
		}})
		analysisPool.Submit(Job{ID: "2", Payload: func() (interface{}, error) {
			log.Debug("starting link counting")
			internal, external := doc.Links(ctx)
			log.Debug("finished link counting")
			return map[string]int{"internal": internal, "external": external}, nil
		}})
		analysisPool.Submit(Job{ID: "3", Payload: func() (interface{}, error) {
			log.Debug("starting heading counting")
			headings := doc.Headings(ctx)
			log.Debug("finished heading counting")
			return headings, nil
		}})
		analysisPool.Submit(Job{ID: "4", Payload: func() (interface{}, error) {
			log.Debug("starting title extraction")
			title := doc.Title(ctx)
			log.Debug("finished title extraction")
			return title, nil
		}})
		analysisPool.Submit(Job{ID: "5", Payload: func() (interface{}, error) {
			log.Debug("starting HTML version detection")
			htmlVersion := doc.HTMLVersion(ctx)
			log.Debug("finished HTML version detection")
			return htmlVersion, nil
		}})
		analysisPool.Submit(Job{ID: "6", Payload: func() (interface{}, error) {
			log.Debug("starting login form check")
			hasLoginForm := doc.HasLoginForm(ctx)
			log.Debug("finished login form check")
			return hasLoginForm, nil
		}})
		analysisPool.Close()
	}()

	for res := range analysisPool.Results() {
		if res.Err != nil {
			log.WithError(res.Err).Error("error during analysis task")
			continue
		}
		switch res.JobID {
		case "1":
			result.InaccessibleLinks = res.Value.(int)
		case "2":
			links := res.Value.(map[string]int)
			result.InternalLinks = links["internal"]
			result.ExternalLinks = links["external"]
		case "3":
			result.Headings = res.Value.(map[string]int)
		case "4":
			result.Title = res.Value.(string)
		case "5":
			result.HTMLVersion = res.Value.(string)
		case "6":
			result.HasLoginForm = res.Value.(bool)
		}
	}
	return nil
}

func parseUrl(_ context.Context, userUrl string) (*url.URL, error) {
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
		return info, errors.New("http status not ok")
	}

	info.bodyByte = bodyByte
	info.responseCode = responseCode

	return info, nil
}

func checkLinksAccessibility(ctx context.Context, links []map[string]interface{}) int {
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	linkCheckFunc := func(ctx context.Context, payload interface{}) (interface{}, error) {
		link := payload.(map[string]interface{})
		linkURL := link["url"].(string)

		req, err := http.NewRequestWithContext(ctx, http.MethodHead, linkURL, nil)
		if err != nil {
			return false, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return false, nil // Treat network errors as inaccessible links
		}
		defer resp.Body.Close()

		return resp.StatusCode < 400, nil
	}

	pool := NewWorkerPool(20, linkCheckFunc)
	pool.Start(ctx)

	go func() {
		for i, link := range links {
			pool.Submit(Job{ID: strconv.Itoa(i), Payload: link})
		}
		pool.Close()
	}()

	inaccessibleCount := 0
	for result := range pool.Results() {
		if result.Err != nil {
			inaccessibleCount++
			continue
		}
		isAccessible := result.Value.(bool)
		if !isAccessible {
			inaccessibleCount++
		}
	}

	return inaccessibleCount
}
