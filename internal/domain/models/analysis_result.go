package models

import (
	"net/url"

	"golang.org/x/net/html"
)

type AnalysisResult struct {
	BaseUrl           *url.URL
	HtmlNode          *html.Node
	BodyByte          []byte
	HTMLVersion       string
	Title             string
	Headings          map[string]int
	InternalLinks     int
	ExternalLinks     int
	InaccessibleLinks int
	HasLoginForm      bool
	Error             string
	StatusCode        int
}
