package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"time"

	"github.com/segmentio/ksuid"

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

func NewConnection(rabbitmqCfg RabbitMQConfig) (*RabbitMQService, error) {
	auth0Token, err := auth.GetAuth0Token(rabbitmqCfg.Auth0Config)
	if err != nil {
		log.Fatalf("Error getting token: %v", err)
	}

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
	

	err = ch.ExchangeDeclare(
		"transaction_requests",
		"fanout",
		true,
		false,
		false,
		false,
		nil,
	)
	failOnError(err, "Failed to declare an exchange")

	return &RabbitMQService{
		rabbitMQConn: conn,
		rabbitMQChan:   ch,
	}, nil

}

func (rmq *RabbitMQService) BroadcastTransaction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	slog.InfoContext(ctx, "broadcasting transaction")
	defer cancel()

	var txnRequest TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&txnRequest); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	txnID := fmt.Sprintf("txn_%s", ksuid.New().String())
	txnRequest.TransactionID = txnID

	responseQueue, err := rmq.createResponseQueue(txnID)
	if err != nil {
		slog.ErrorContext(ctx, "error creating response queue","error", err.Error())
		http.Error(w, "Failed to create response queue", http.StatusInternalServerError)
		return
	}

	if err := rmq.publishTransaction(ctx, txnRequest); err != nil {
		http.Error(w, "Failed to publish message", http.StatusInternalServerError)
		return
	}

	isCompliant, err := rmq.collectResponses(ctx, responseQueue.Name, txnID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, map[string]bool{"is_compliant": isCompliant})
}

func (rmq *RabbitMQService) createResponseQueue(txnID string) (amqp.Queue, error) {
	return rmq.rabbitMQChan.QueueDeclare(
		txnID, // name
		false, // durable
		false, // auto delete
		true,  // exclusive -> important to ensure that no one else can recieve responses from this transaction responses queue 
		false,
		nil,
	)
}

func  (rmq *RabbitMQService) publishTransaction(ctx context.Context, txnRequest TransactionRequest) error {
	txnRequestByte, err := json.Marshal(txnRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction request: %w", err)
	}

	return rmq.rabbitMQChan.PublishWithContext(
		ctx,
		"transaction_requests",
		"",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        txnRequestByte,
		},
	)
}

func (rmq *RabbitMQService) collectResponses(ctx context.Context, queueName, transactionID string) (bool, error) {
	slog.InfoContext(ctx, "collecting responses for transaction", "transaction_id", transactionID)
	responseChan, err := rmq.rabbitMQChan.Consume(
		transactionID,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return false, fmt.Errorf("failed to set up response channel: %w", err)
	}

	responses := 0
	for {
		select {
		case response := <-responseChan:
			if response.RoutingKey == transactionID {
				var txnResponse TransactionResponse
				if err := json.Unmarshal(response.Body, &txnResponse); err != nil {
					return false, fmt.Errorf("failed to unmarshal response: %w", err)
				}

				slog.InfoContext(ctx, "received response", "transaction_id", transactionID, "is_valid", txnResponse.IsValid)

				if txnResponse.IsValid {
					responses++
				}
				if responses >= 5 {
					rmq.deleteQueue(ctx, queueName)
					return true, nil
				}
			}
		case <-ctx.Done():
			rmq.deleteQueue(ctx, queueName)
			return false, nil
		}
	}
}

func (rmq *RabbitMQService) deleteQueue(ctx context.Context, queueName string) {
	_, err := rmq.rabbitMQChan.QueueDelete(queueName, false, false, false)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete queue", "queueName", queueName, "error",err)
	}
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func (rmq *RabbitMQService) Close() {
	rmq.rabbitMQChan.Close()
	rmq.rabbitMQConn.Close()
}

func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
