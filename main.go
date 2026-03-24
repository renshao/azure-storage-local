package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/azure-storage-local/internal/api"
	"github.com/azure-storage-local/internal/queue"
	"github.com/azure-storage-local/internal/web"
)

func main() {
	store := queue.NewStore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start TTL expiration worker
	queue.StartTTLWorker(ctx, store, 1*time.Second)

	// Queue API server on port 10001
	apiHandler := api.Router(store)
	apiServer := &http.Server{
		Addr:    ":10001",
		Handler: apiHandler,
	}

	// Web UI server on port 10011
	webHandler := web.Server(store)
	webServer := &http.Server{
		Addr:    ":10011",
		Handler: webHandler,
	}

	// Start servers
	go func() {
		fmt.Println("Queue API listening on http://127.0.0.1:10001/devstoreaccount1")
		if err := apiServer.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "API server error: %v\n", err)
			os.Exit(1)
		}
	}()

	go func() {
		fmt.Println("Web UI  listening on http://127.0.0.1:10011")
		if err := webServer.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Web server error: %v\n", err)
			os.Exit(1)
		}
	}()

	fmt.Println()
	fmt.Println("Connection string:")
	fmt.Println("  DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;QueueEndpoint=http://127.0.0.1:10001/devstoreaccount1;")
	fmt.Println()

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	fmt.Println("\nShutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	apiServer.Shutdown(shutdownCtx)
	webServer.Shutdown(shutdownCtx)
	fmt.Println("Stopped.")
}
