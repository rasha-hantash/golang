# Build stage
FROM golang:1.23.1 AS build
WORKDIR /app
COPY . .
RUN go mod download && go mod tidy
# Change working directory to where the Go files are
WORKDIR /app/operator
# Build the operator
RUN go build -v -o /usr/local/bin/operator

# Final stage
FROM golang:1.23.1
COPY --from=build /usr/local/bin/operator /usr/local/bin/operator
ENTRYPOINT ["operator"]