# Use a specific version of golang alpine
FROM golang:1.23.1-alpine

# Install protoc and related tools
RUN apk add --no-cache protobuf protobuf-dev git

# Set the working directory
WORKDIR /app

# Install Go plugins for protobuf and gRPC
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

# Copy go mod files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the rest of the project
COPY . .

# Generate gRPC code
RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    proto/service.proto

WORKDIR /app/server
# Build the application
RUN go build -o /usr/local/bin/server 


# Install grpc_health_probe
RUN GRPC_HEALTH_PROBE_VERSION=v0.3.6 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

# Expose the port the app runs on
EXPOSE 50051

# Run the server binary
CMD ["/usr/local/bin/server"]