package main

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	avsv1 "yourproject/avsv1" // Replace with the actual import path of your generated proto package
)



func main() {
	// Set up a connection to the server.
	// List of localhost addresses (ports)
	addresses := []string{
		"localhost:50051",
		"localhost:50052",
		"localhost:50053",
	}

	// Try to connect to each address
	var client avsv1.TaskSubmissionServiceClient
	var conn *grpc.ClientConn
	var err error

	for _, address := range addresses {
		conn, err = grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			defer conn.Close()
			client = avsv1.NewTaskSubmissionServiceClient(conn)
			log.Printf("Connected to %s", address)
			break
		}
		log.Printf("Failed to connect to %s: %v", address, err)
	}

	if client == nil {
		log.Fatalf("Failed to connect to any address")
	}

	// Set up a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Prepare the request
	req := &avsv1.SubmitTaskRequest{
		Task: &avsv1.Task{
			Id: "task-123",
			Params: &avsv1.Params{
				To:                   [][]byte{[]byte("recipient1"), []byte("recipient2")},
				From:                 [][]byte{[]byte("sender1"), []byte("sender2")},
				Data:                 []byte("task data"),
				Value:                []byte("100"),
				QuorumThresholdCount: 2,
				PolicyId:             "policy-456",
				BlockExpiration:      uint64(time.Now().Add(24 * time.Hour).Unix()),
				ApiKeys:              []string{"key1", "key2"},
			},
		},
	}

	// Call the SubmitTask RPC
	resp, err := client.SubmitTask(ctx, req)
	if err != nil {
		log.Fatalf("could not submit task: %v", err)
	}

	// Process the response
	log.Printf("Task submitted successfully. Task ID: %s, Status: %s", resp.TaskId, resp.Status)
}