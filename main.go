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

	"github.com/RishangS/auth-service/utils"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/RishangS/auth-service/proto" // Update with your proto package path
)

const (
	grpcPort = "50051"
	httpPort = "8080"
)

// authServer implements the AuthServiceServer interface from your proto file
type authServer struct {
	pb.UnimplementedAuthServiceServer
	authClient *utils.AuthClient
}

func NewAuthServer() *authServer {
	return &authServer{
		authClient: utils.NewAuthClient(),
	}
}

func main() {
	// Create context that listens for interrupt signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize auth server
	authServer := NewAuthServer()

	// Create gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterAuthServiceServer(grpcServer, authServer)
	reflection.Register(grpcServer) // Enable reflection for testing with grpcurl

	// Start gRPC server
	go func() {
		lis, err := net.Listen("tcp", ":"+grpcPort)
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Create gRPC-Gateway mux
	gwMux := runtime.NewServeMux()

	// Register gRPC-Gateway endpoints
	opts := []grpc.DialOption{grpc.WithInsecure()}
	err := pb.RegisterAuthServiceHandlerFromEndpoint(ctx, gwMux, "localhost:"+grpcPort, opts)
	if err != nil {
		log.Fatalf("failed to register gateway: %v", err)
	}

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    ":" + httpPort,
		Handler: gwMux,
	}

	// Start HTTP server
	go func() {
		log.Printf("HTTP server listening on :%s", httpPort)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
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
	ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("HTTP server forced to shutdown: %v", err)
	}

	log.Println("Servers exited properly")
}
