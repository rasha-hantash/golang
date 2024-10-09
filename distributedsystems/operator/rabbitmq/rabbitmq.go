package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"log/slog"

	"time"

	"github.com/rasha-hantash/golang/distributedsystems/libs/auth"
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


type RabbitMQService struct {
	rabbitMQConn *amqp.Connection
	rabbitMQChan   *amqp.Channel

}

type RabbitMQConfig struct {
	Host     string
	Port     string
	Auth0Config auth.Auth0Config
}



func NewConnection(rabbitmqCfg RabbitMQConfig) (*RabbitMQService) {
	auth0Token, err := auth.GetAuth0Token(rabbitmqCfg.Auth0Config)
	failOnError(err, "Error getting token")

	rabbitmqURL := fmt.Sprintf("amqp://%s:5672", rabbitmqCfg.Host)

	// Create a custom dialer that includes the OAuth2 token
	conn, err := amqp.DialConfig(rabbitmqURL, amqp.Config{
		Heartbeat: 10 * time.Second,
		Locale:    "en_US",
		SASL:      []amqp.Authentication{&amqp.PlainAuth{Username: "", Password: auth0Token}},
	})
	failOnError(err, "Failed to open a connection")
	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")

	return &RabbitMQService{
		rabbitMQConn: conn,
		rabbitMQChan:   ch,
	}

}

func (rmq *RabbitMQService) Setup() error {
	q, err := rmq.rabbitMQChan.QueueDeclare(
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

	// todo check to see if i can create the same transaction_requests exchange 
	// and txn_{id} that dispatcher creates
	err = rmq.rabbitMQChan.QueueBind(
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

func (rmq *RabbitMQService) ProcessTransactions(ctx context.Context) error {
	// how 
	msgs, err := rmq.rabbitMQChan.ConsumeWithContext(
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
		rmq.processTransaction(ctx, d)
		d.Ack(false)
	}

	return nil
}

func (rmq *RabbitMQService) processTransaction(ctx context.Context, d amqp.Delivery) {
	var txnRequest TransactionRequest
	if err := json.Unmarshal(d.Body, &txnRequest); err != nil {
		slog.ErrorContext(ctx, "error decoding transaction", "error", err.Error())
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

	if err := rmq.publishResponse(ctx, response); err != nil {
		slog.ErrorContext(ctx, "error publishing response", "error", err.Error())
	} else {
		slog.InfoContext(ctx, "published response for transaction", "transaction_id", txnRequest.TransactionID)
	}
}

func (rmq *RabbitMQService) publishResponse(ctx context.Context, response TransactionResponse) error {
	responseBody, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("error encoding response: %w", err)
	}

	err = rmq.rabbitMQChan.PublishWithContext(
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

func (rmq *RabbitMQService) Close() {
	rmq.rabbitMQChan.Close()
	rmq.rabbitMQConn.Close()
}


func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}
