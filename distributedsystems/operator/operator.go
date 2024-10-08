package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"time"
	"net/http"
	"bytes"
	"io"
	"github.com/spf13/viper"

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

func NewTransactionProcessor(connectionString, auth0Token string) (*TransactionProcessor, error) {
	// Create a custom dialer that includes the OAuth2 token
	conn, err := amqp.DialConfig(connectionString, amqp.Config{
		Heartbeat: 10 * time.Second,
		Locale:    "en_US",
		SASL:      []amqp.Authentication{&amqp.PlainAuth{Username: "", Password: auth0Token}},
	})
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

	// todo check to see if i can create the same transaction_requests exchange 
	// and txn_{id} that dispatcher creates
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
	// how 
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
	viper.SetConfigFile("../.env")
    err := viper.ReadInConfig()
    if err != nil {
        fmt.Printf("Error reading config file: %s\n", err)
    }

	auth0Token, err := getAuth0Token()
	if err != nil {
		log.Fatal(err)
	}
	// todo put the rabbitmq hostname in env var
	connectionString := "amqp://localhost@localhost:5672/"

	processor, err := NewTransactionProcessor(connectionString, auth0Token)
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


func getAuth0Token() (string, error) {
	url  := viper.GetString("AUTH0_DOMAIN") + "/oauth/token"
	payload := map[string]string{
		"client_id":     viper.GetString("DISPATCHER_AUTH0_CLIENT_ID"),
		"client_secret": viper.GetString("DISPATCHER_AUTH0_CLIENT_SECRET"),
		"audience":      "rabbitmq",
		"grant_type":    "client_credentials",
	}


	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshaling payload: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d, body: %s", res.StatusCode, string(body))
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}

	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		return "", fmt.Errorf("error unmarshaling response: %v", err)
	}

	return tokenResponse.AccessToken, nil
}