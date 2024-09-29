package main

import (
    "context"
    "log"
    "time"

    "google.golang.org/grpc"
    pb "github.com/rasha-hantash/golang/concurrentgrpccalls/proto"
)

const (
    address = "server:50051"
)

func main() {
    conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
    if err != nil {
        log.Fatalf("did not connect: %v", err)
    }
    defer conn.Close()
    c := pb.NewHealthServiceClient(conn)

    for {
        submitHealth(c)
        time.Sleep(30 * time.Second)
    }
}

func submitHealth(c pb.HealthServiceClient) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    r, err := c.SubmitHealth(ctx, &pb.HealthRequest{ClientId: "client1"})
    if err != nil {
        log.Printf("could not submit health: %v", err)
    } else {
        log.Printf("Health Status: %s", r.GetStatus())
    }
}