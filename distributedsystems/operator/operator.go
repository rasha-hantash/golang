package main

import (
	"context"

	"fmt"
	"log"
	"net"

	avsv1 "github.com/rasha-hantash/golang/distributedsystems/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"math/rand"
	"time"
)


type server struct {
	avsv1.UnimplementedTransactionServiceServer // todo look up what unimplemented does
}

func main() {
	// Define the port to listen on
	port := 50051

	// Create a TCP listener
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Create a new gRPC server
	s := grpc.NewServer()

	// Create an instance of your server struct
	taskServer := &server{}

	// Register your service with the gRPC server
	avsv1.RegisterTransactionServiceServer(s, taskServer)

	// Start the server
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}



func (s *server) SubmitTask(ctx context.Context, req *avsv1.SubmitTransactionRequest) (*avsv1.SubmitTransactionResponse, error) {
	// Validate the request

	if req.Transaction == nil || req.Transaction.Params == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid task or params")
	}

	// Process the task
	// This is where you would implement your business logic
	// For example, you might save the task to a database, initiate some processing, etc.
	// Generate a random duration between 0 and 4 seconds
    duration := time.Duration(rand.Intn(8001)) * time.Millisecond

    // Simulate processing time
    time.Sleep(duration)
	log.Printf("Task processed after: %f seconds", duration.Seconds())


	// Return a response
	return &avsv1.SubmitTransactionResponse{
		TransactionId: req.Transaction.Id,
		Status: "Weeeee",
	}, nil
}