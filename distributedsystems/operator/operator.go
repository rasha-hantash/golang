package main

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	avsv1 "path/to/your/proto/package"
	"google.golang.org/grpc"
	"fmt"
)

type server struct {
	avsv1.UnimplementedYourServiceServer
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
	avsv1.RegisterYourServiceServer(s, taskServer)

	// Start the server
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}



func (s *server) SubmitTask(ctx context.Context, req *avsv1.SubmitTaskRequest) (*avsv1.SubmitTaskResponse, error) {
	// Validate the request
	if req.Task == nil || req.Task.Params == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid task or params")
	}

	// Process the task
	// This is where you would implement your business logic
	// For example, you might save the task to a database, initiate some processing, etc.

	// Return a response
	return &avsv1.SubmitTaskResponse{
		TaskId: req.Task.Id,
		Status: "Submitted",
	}, nil
}