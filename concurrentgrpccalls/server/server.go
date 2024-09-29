package main

import (
    "context"
    "log"
    "net"
    "database/sql"
    _ "github.com/lib/pq"
    "fmt"

    "google.golang.org/grpc"
    pb "github.com/rasha-hantash/golang/concurrentgrpccalls/proto"
)


const (
    host     = "postgres"
    port     = 5432
    user     = "postgres"
    password = "postgres"
    dbname   = "postgres"
)

func getOutboundIP() string {
    conn, err := net.Dial("udp", "8.8.8.8:80")
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    localAddr := conn.LocalAddr().(*net.UDPAddr)

    return localAddr.IP.String()
}

type server struct {
    pb.UnimplementedHealthServiceServer
}

func (s *server) SubmitHealth(ctx context.Context, in *pb.HealthRequest) (*pb.HealthResponse, error) {
    log.Printf("Received health check from client: %v", in.GetClientId())
    return &pb.HealthResponse{Status: "OK"}, nil
}

func main() {
        // Construct connection string
        psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
        "password=%s dbname=%s sslmode=disable",
        host, port, user, password, dbname)

    // Open database connection
    db, err := sql.Open("postgres", psqlInfo)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Test the connection
    err = db.Ping()
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Successfully connected to the database!")

    // Get the host IP
    hostIP := getOutboundIP()

    // Assume a port for the service
    servicePort := "50051" // Change this to your actual service port

    // Combine IP and port
    hostIPAndPort := fmt.Sprintf("%s:%s", hostIP, servicePort)

    // Insert the host IP and port into the database
    _, err = db.Exec(`
        INSERT INTO servers (host_ip_and_port)
        VALUES ($1)
        ON CONFLICT (host_ip_and_port) DO NOTHING
    `, hostIPAndPort)
    if err != nil {
        log.Fatal(err)
    }



    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }
    s := grpc.NewServer()
    pb.RegisterHealthServiceServer(s, &server{})
    log.Printf("Server listening at %v", lis.Addr())
    if err := s.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}