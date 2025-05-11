package main

import (
	"context"
	"web_page_analyzer/internal/http"
	log "github.com/sirupsen/logrus"
	 _ "net/http/pprof"
)

func main() {

	logInstance :=  log.New()
	logInstance.SetLevel(log.DebugLevel)
	logInstance.SetFormatter(&log.JSONFormatter{})

	// Get context
	ctx := context.Background()

	// Init HTTP
	http.Init(ctx, logInstance)
}