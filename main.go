package main

import (
	"log"
	"net"
	"net/http"

	"github.com/RishangS/auth-service/handlers"
	pb "github.com/RishangS/auth-service/proto"
	"google.golang.org/grpc"
)

type authServer struct {
	pb.UnimplementedAuthServiceServer
}

func main() {
	// Start gRPC server
	go func() {
		listener, err := net.Listen("tcp", ":50051")
		if err != nil {
			log.Fatalf("gRPC listen failed: %v", err)
		}

		grpcServer := grpc.NewServer()
		pb.RegisterAuthServiceServer(grpcServer, &handlers.AuthServer{})

		log.Println("gRPC server running on :50051")
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("gRPC serve failed: %v", err)
		}
	}()

	// Start HTTP server
	http.HandleFunc("/login", handlers.LoginHandler)
	log.Println("HTTP server running on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("HTTP serve failed: %v", err)
	}
}
