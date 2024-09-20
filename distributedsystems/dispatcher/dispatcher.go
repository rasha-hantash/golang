package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	// "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	pb "github.com/rasha-hantash/golang/distributedsystems/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Assuming you have a generated protobuf file, import it like this:
// import pb "your_module/proto"

type Dispatcher struct {
    docker *client.Client
    port   int
    // ... other fields
}

func main() {
	dispatcher, err := NewDispatcher()
	if err != nil {
		fmt.Printf("Failed to create dispatcher: %v\n", err)
		return
	}
	

	// Prepare the request
	request := &pb.SubmitTransactionRequest{
		Transaction: &pb.Transaction{
			Id: "txn-123",
			Params: &pb.Params{
				To:   [][]byte{[]byte("recipient1"), []byte("recipient2")},
				From: [][]byte{[]byte("sender1"), []byte("sender2")},
				Data: []byte("task data"),
			},

			// Add any other fields as needed
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	errArray := dispatcher.DispatchToAll(ctx, request)
	fmt.Println(errArray)
}


func NewDispatcher() (*Dispatcher, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Dispatcher{
		docker: cli,
		port:   50051, // Use the default port for gRPC
	}, nil
}

func (d *Dispatcher) DispatchToAll(ctx context.Context, request *pb.SubmitTransactionRequest) []error {
    fmt.Println("Dispatching to all containers...")
    containers, err := d.docker.ContainerList(ctx, container.ListOptions{})
    if err != nil {
        return []error{fmt.Errorf("failed to list containers: %w", err)}
    }

    var wg sync.WaitGroup
    errChan := make(chan error, len(containers))  // Buffered channel to avoid blocking

    for _, container := range containers {
        if container.Labels["com.docker.compose.service"] == "api" {
            wg.Add(1)
            go func(containerID string) {
                defer wg.Done()
                reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
                defer cancel()
				address := fmt.Sprintf("localhost:%d",int(container.Ports[0].PublicPort)) // Use the default port

                if err := d.sendGRPCRequest(reqCtx, containerID, request, address); err != nil {
                    errChan <- err
                }
            }(container.ID)
        }
    }

    // Close the error channel when all goroutines are done
    go func() {
        wg.Wait()
        close(errChan)
    }()

    // Collect errors
    var errors []error
    for err := range errChan {
        errors = append(errors, err)
    }

    return errors
}

func (d *Dispatcher) sendGRPCRequest(ctx context.Context, containerID string, request *pb.SubmitTransactionRequest, address string) error {
	// address := fmt.Sprintf("api:%d", port) // Use the provided port
	fmt.Println("address: ", address)

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", containerID[:12], err)
	}
	defer conn.Close()


	client := pb.NewTransactionServiceClient(conn)

	resp, err := client.SubmitTransaction(ctx, request)
	if err != nil {
		return fmt.Errorf("gRPC call failed for %s: %w", containerID[:12], err)
	}

	// Handle the response as needed
	fmt.Printf("Response from %s: %v\n", containerID[:12], resp)

	return nil
}