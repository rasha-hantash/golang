package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/rasha-hantash/golang/distributedsystems/libs/auth"
	"github.com/rasha-hantash/golang/distributedsystems/operator/config"
	"github.com/rasha-hantash/golang/distributedsystems/operator/rabbitmq"
	"github.com/rasha-hantash/golang/distributedsystems/libs/logger"
)





func main() {

	// load configuration 
	h := &logger.ContextHandler{Handler: slog.NewJSONHandler(os.Stdout, nil)}
	slog.SetDefault(slog.New(h))
    ctx := logger.AppendCtx(context.Background(), slog.String("request_id", "req-123"))


	cfg , err := config.LoadConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize RabbitMQ connection
	rabbitCfg := rabbitmq.RabbitMQConfig{
		Host: cfg.RabbitMQHost,
		Port: "5672", // Assuming default port, adjust if needed
		Auth0Config: auth.Auth0Config{
			Domain:       cfg.Auth0Domain,
			ClientID:     cfg.OperatorClientID,
			ClientSecret: cfg.OperatorClientSecret,
			Audience:     "rabbitmq",
		},
	}
	rabbitmqSvc := rabbitmq.NewConnection(rabbitCfg)
	defer rabbitmqSvc.Close()

	if err := rabbitmqSvc.Setup(); err != nil {
		log.Fatalf("Failed to set up processor: %v", err)
	}

	slog.InfoContext(ctx, "operator service is now listening for messages")


	if err := rabbitmqSvc.ProcessTransactions(ctx); err != nil {
		slog.ErrorContext(ctx, "Error processing transactions", "error", err.Error())
		log.Fatalf("Error processing transactions: %v", err)
	}
}

// todo: set up identity pool and iam for the operator service
// todo: fix logging with slog