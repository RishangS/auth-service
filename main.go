package main

import (
	"context"
	"log"
	"net"
	"net/http"

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

	// Add health check endpoint to gwMux
	gwMux.HandlePath("GET", "/health", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    ":" + httpPort,
		Handler: gwMux,
	}

	// Start HTTP server
	log.Printf("HTTP server listening on :%s", httpPort)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("failed to serve: %v", err)
	}

}
