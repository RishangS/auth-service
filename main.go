package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	auth "github.com/RishangS/auth-service/gen/proto"
	"github.com/RishangS/auth-service/handler"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

const (
	grpcPort = "50051"
	httpPort = "8080"
)

func main() {
	// Create context that listens for interrupt signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize auth server
	authServer := handler.NewAuthHandler()

	// Create gRPC server
	grpcServer := grpc.NewServer()
	auth.RegisterAuthServiceServer(grpcServer, authServer)
	reflection.Register(grpcServer) // Enable reflection for testing with grpcurl

	// Start gRPC server
	grpcLis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	go func() {
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := grpcServer.Serve(grpcLis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Create gRPC-Gateway mux
	gwMux := runtime.NewServeMux()

	// Register gRPC-Gateway endpoints
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()), // Updated to use new API
	}
	err = auth.RegisterAuthServiceHandlerFromEndpoint(ctx, gwMux, "localhost:"+grpcPort, opts)
	if err != nil {
		log.Fatalf("failed to register gateway: %v", err)
	}

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    ":" + httpPort,
		Handler: gwMux,
	}

	// Add health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Start HTTP server
	go func() {
		log.Printf("HTTP server listening on :%s", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down servers...")

	// Shutdown gRPC server
	grpcServer.GracefulStop()

	// Shutdown HTTP server with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server forced to shutdown: %v", err)
	}

	log.Println("Servers exited properly")
}
