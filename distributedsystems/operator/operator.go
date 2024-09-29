package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type TransactionRequest struct {
	TransactionID string `json:"transaction_id"`
	TxnHash       string `json:"txn_hash"`
	From          string `json:"from"`
	To            string `json:"to"`
	Value         int64  `json:"value"`
}

type TransactionResponse struct {
	TransactionID string `json:"transaction_id"`
	IsValid       bool   `json:"is_valid"`
}

type TransactionProcessor struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewTransactionProcessor(connectionString string) (*TransactionProcessor, error) {
	conn, err := amqp.Dial(connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	return &TransactionProcessor{
		conn: conn,
		ch:   ch,
	}, nil
}

func (tp *TransactionProcessor) Setup() error {
	err := tp.ch.ExchangeDeclare(
		"transaction_requests",
		"fanout",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare an exchange: %w", err)
	}

	q, err := tp.ch.QueueDeclare(
		"",
		false,
		false,
		true,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare a queue: %w", err)
	}

	err = tp.ch.QueueBind(
		q.Name,
		"",
		"transaction_requests",
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind a queue: %w", err)
	}

	return nil
}

func (tp *TransactionProcessor) ProcessTransactions(ctx context.Context) error {
	msgs, err := tp.ch.ConsumeWithContext(
		ctx,
		"",
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	for d := range msgs {
		tp.processTransaction(ctx, d)
		d.Ack(false)
	}

	return nil
}

func (tp *TransactionProcessor) processTransaction(ctx context.Context, d amqp.Delivery) {
	var txnRequest TransactionRequest
	if err := json.Unmarshal(d.Body, &txnRequest); err != nil {
		log.Printf("Error decoding transaction: %v", err)
		return
	}

	// Simulate processing time
	time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

	// Simulate validation (replace with actual validation logic)
	isValid := rand.Float32() < 0.9

	slog.InfoContext(ctx, "Processing transaction",
		"transaction_id", txnRequest.TransactionID,
		"is_valid", isValid,
	)

	response := TransactionResponse{
		TransactionID: txnRequest.TransactionID,
		IsValid:       isValid,
	}

	if err := tp.publishResponse(ctx, response); err != nil {
		log.Printf("Error publishing response: %v", err)
	} else {
		log.Printf("Published response for transaction %s", txnRequest.TransactionID)
	}
}

func (tp *TransactionProcessor) publishResponse(ctx context.Context, response TransactionResponse) error {
	responseBody, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error encoding response: %w", err)
	}

	err = tp.ch.PublishWithContext(
		ctx,
		"",
		response.TransactionID,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        responseBody,
		},
	)
	if err != nil {
		return fmt.Errorf("error publishing response: %w", err)
	}

	return nil
}

func (tp *TransactionProcessor) Close() {
	tp.ch.Close()
	tp.conn.Close()
}

func main() {
	connectionString := "amqp://your_username:your_password@rabbitmq:5672/"

	processor, err := NewTransactionProcessor(connectionString)
	if err != nil {
		log.Fatalf("Failed to create transaction processor: %v", err)
	}
	defer processor.Close()

	if err := processor.Setup(); err != nil {
		log.Fatalf("Failed to set up processor: %v", err)
	}

	ctx := context.Background()

	fmt.Println("Transaction processor is waiting for messages. To exit press CTRL+C")

	if err := processor.ProcessTransactions(ctx); err != nil {
		log.Fatalf("Error processing transactions: %v", err)
	}
}