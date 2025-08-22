package main

import (
	"context"
	_ "net/http/pprof"
	"web_page_analyzer/cmd/app"

	log "github.com/sirupsen/logrus"
)

func main() {
	ctx := context.Background()
	a, cleanup, err := app.InitializeApp(ctx)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize app")
	}
	defer cleanup()

	a.Run()
}
