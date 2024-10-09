package main

import (
	"context"
	"fmt"
	"log"

	"github.com/rasha-hantash/golang/distributedsystems/operator/config"
	"github.com/rasha-hantash/golang/distributedsystems/libs/auth"
	"github.com/rasha-hantash/golang/distributedsystems/operator/rabbitmq"
)





func main() {
	cfg , err := config.LoadConfig()
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
	rabbitmqSvc, err := rabbitmq.NewConnection(rabbitCfg)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitmqSvc.Close()

	if err := rabbitmqSvc.Setup(); err != nil {
		log.Fatalf("Failed to set up processor: %v", err)
	}

	ctx := context.Background()

	fmt.Println("Transaction processor is waiting for messages. To exit press CTRL+C")

	if err := rabbitmqSvc.ProcessTransactions(ctx); err != nil {
		log.Fatalf("Error processing transactions: %v", err)
	}
}