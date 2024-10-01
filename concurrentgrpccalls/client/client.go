package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
    "database/sql"
	"sync"
	"time"
    _ "github.com/lib/pq"

	pb "github.com/rasha-hantash/golang/concurrentgrpccalls/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
    // Establish database connection
    connStr := "host=postgres port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
    
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        log.Fatalf("Failed to open database: %v", err)
    }
    defer db.Close()

    // Verify the connection
    err = db.Ping()
    if err != nil {
        log.Fatalf("Failed to ping database: %v", err)
    }

    // Call the function
    if err := checkAllServers(db); err != nil {
        log.Fatalf("Error checking servers: %v", err)
    }
}

func checkAllServers(db *sql.DB) error {
    // Query to fetch all server addresses
    rows, err := db.Query("SELECT host_ip_and_port FROM servers")
    if err != nil {
        return fmt.Errorf("error querying servers: %v", err)
    }
    defer rows.Close()

    var wg sync.WaitGroup
    for rows.Next() {
        var address string
        if err := rows.Scan(&address); err != nil {
            return fmt.Errorf("error scanning row: %v", err)
        }

        wg.Add(1)
        go func(addr string) {
            defer wg.Done()
            submitHealth(addr)
        }(address)
    }

    if err := rows.Err(); err != nil {
        return fmt.Errorf("error iterating rows: %v", err)
    }

    submitHealth("172.19.0.28:50051")
    wg.Wait()
    return nil
}

func submitHealth(address string) {
    slog.Info("submitting health", "address", address)
    conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        slog.Error(err.Error())
        return
    }
    defer conn.Close()
    
    c := pb.NewHealthServiceClient(conn)
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    r, err := c.SubmitHealth(ctx, &pb.HealthRequest{ClientId: "client-id"})
    if err != nil {
       slog.ErrorContext(ctx, err.Error())
    } else {
       slog.InfoContext(ctx, "success", "address", address,  "health_status", r.GetStatus())
    }
}