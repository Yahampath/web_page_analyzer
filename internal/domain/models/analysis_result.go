package models

type AnalysisResult struct{
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