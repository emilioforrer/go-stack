package httpsvr_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/emilioforrer/go-stack/pkg/httpsvr"
)

func ExampleNewDefaultHTTPServer() {
	// Create a new router
	router := httpsvr.NewDefaultRouter()
	router.Use(httpsvr.RequestLoggerMiddleware(true))

	// Register some test routes
	router.GET("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Welcome to the server!")
	})

	// Create server config with the router as handler
	config := httpsvr.NewDefaultConfig(router)

	// Create the HTTP server
	server := httpsvr.NewDefaultHTTPServer(config)

	// Create a context for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Channel to listen for interrupt signals
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	// Wait for interrupt signal (remove this if you want to run the server indefinitely)
	go func() {
		time.Sleep(1 * time.Second) // Wait 1 seconds
		done <- syscall.SIGTERM     // Send termination signal
	}()

	fmt.Println("Server is running...")

	// Start server in a goroutine
	go func() {
		fmt.Printf("Starting server on %s\n", config.Addr)
		if err := server.Start(ctx); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	// Block until interrupt signal is received
	<-done
	fmt.Println("Server is shutting down...")

	// Initiate graceful shutdown
	if err := server.Stop(ctx); err != nil {
		fmt.Printf("Server shutdown error: %v\n", err)
	}

	fmt.Println("Server stopped gracefully")

	// Output:
	// Server is running...
	// Starting server on :4000
	// Server is shutting down...
	// Server stopped gracefully
}

func TestNewDefaultHTTPServer(t *testing.T) {
	router := httpsvr.NewDefaultRouter()
	config := httpsvr.NewDefaultConfig(router)
	server := httpsvr.NewDefaultHTTPServer(config)

	if server == nil {
		t.Error("Expected server to not be nil")
	}
}
