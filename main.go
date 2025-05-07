package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"os"
	"syscall"
	"time"
	"web_page_analyzer/internal/application/config"
	"github.com/gorilla/mux"
)

func main() {
	cfg, err := config.NewAppConfig()
	if err != nil {
		log.Fatal(fmt.Sprintf(`Failed to lod config: %v`, err))
	}

	// Get context
	ctx := context.Background()

	// Setup HTTP server
	httpServer := setupHttpServer(ctx, cfg)

	// Start HTTP server
	go startHTTPServer(ctx, httpServer)

	// Wait for shutdown
	waitForShutdown(ctx, httpServer)
}

func setupHttpServer(_ context.Context, cfg *config.AppConfig) *http.Server {
	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello World!")
	}).Methods("GET")

	return &http.Server{
		Addr:    cfg.HttpServerAddr,
		Handler: router,
	}
}

func startHTTPServer(_ context.Context, server *http.Server) {
	log.Println("Starting HTTP server on", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func waitForShutdown(ctx context.Context, httpServer *http.Server) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(ctx, 30 * time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server exiting")
}
