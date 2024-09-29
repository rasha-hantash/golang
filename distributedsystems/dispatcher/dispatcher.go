package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"golang.org/x/time/rate"
	"log/slog"
	"log"
	"net/http"

	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/segmentio/ksuid"
)

type TransactionRequest struct {
	TransactionID string `json:"transaction_id"`
	TxnHash       string `json:"txn_hash"`
	From          string `json:"from"`
	To            string `json:"to"`
	Value         int64    `json:"value"` // amount sent from the sender to the receiver
}

type TransactionResponse struct {
	TransactionID string `json:"transaction_id"`
	IsValid   bool   `json:"is_valid"`
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

type server struct {
	rabbitMQConn     *amqp.Connection
	rabbitMQChan     *amqp.Channel
}

func newServer(conn *amqp.Connection, ch *amqp.Channel) *server {
	return &server{
		rabbitMQConn:     conn,
		rabbitMQChan:     ch,
	}
}

func main() {
	// Connect to RabbitMQ
	conn, err := amqp.Dial("amqp://your_username:your_password@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		"transaction_requests",   // name
		"fanout", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	failOnError(err, "Failed to declare an exchange")

	// // Declare the transactions queue
	// transQueue, err := ch.QueueDeclare(
	// 	"transaction_requests", // name
	// 	false,                  // durable
	// 	false,                  // delete when unused
	// 	false,                  // exclusive
	// 	false,                  // no-wait
	// 	nil,                    // arguments
	// )
	// failOnError(err, "Failed to declare transactions queue")

	// Create a new server instance
	s := newServer(conn, ch)

	// Initialize and start the gateway
	router := mux.NewRouter()

	// Initialize rate limiter
	// limiter := rate.NewLimiter(rate.Limit(2), 10) // 2 requests per second, allow bursts up to 10

	// Add rate limiter middleware
	// router.Use(rateLimiterMiddleware(limiter))

	// Add health check endpoint
	router.HandleFunc("/health", healthCheckHandler).Methods("GET")

	// Add your routes here, for example:
	router.HandleFunc("/transaction", s.BroadcastTransaction).Methods("POST")

	// Define the port
	port := "8080"
	// Start the HTTP server
	// Start the HTTP server
	log.Printf("Server is now listening on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func rateLimiterMiddleware(limiter *rate.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *server) BroadcastTransaction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Attempt to unmarshal the body into the TransactionRequest struct
	var txnRequest TransactionRequest
	// Decode JSON directly from the request body
	if err := json.NewDecoder(r.Body).Decode(&txnRequest); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Generate and set the TransactionID
	txnID := fmt.Sprintf("txn_%s", ksuid.New().String())
	txnRequest.TransactionID = txnID
	// Create a unique response queue for this transaction
	responseQueue, err := s.rabbitMQChan.QueueDeclare(
		txnID, // queue name
		false,         // durable
		false,         // delete when unused
		true,          // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		http.Error(w, "Failed to create response queue", http.StatusInternalServerError)
		return
	}


	txnRequestByte, err := json.Marshal(txnRequest)
	if err != nil {
		http.Error(w, "Failed to marshal transaction request", http.StatusInternalServerError)
		return
	}

	// Publish the transaction to RabbitMQ
	err = s.rabbitMQChan.PublishWithContext(
		ctx,
		"transaction_requests",           // exchange
		"", // routing key (queue name)
		false,              // mandatory
		false,              // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			// CorrelationId: txnRequest.TransactionID,
			// ReplyTo:       responseQueue.Name, // todo look into what this does
			Body: txnRequestByte,
		})
	if err != nil {
		http.Error(w, "Failed to publish message", http.StatusInternalServerError)
		return
	}

	isCompliant, err := s.collectResponses(ctx, responseQueue.Name, txnID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]bool{"is_compliant": isCompliant}
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to create response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

func (s *server) collectResponses(ctx context.Context, queueName, transactionID string) (bool, error) {
	slog.InfoContext(ctx, "collecting responses for transaction", "transaction_id", transactionID)
	responseChan, err := s.rabbitMQChan.Consume(
		transactionID, // queue
		"",            // consumer
		true,          // auto-ack
		false,         // exclusive
		false,         // no-local
		false,         // no-wait
		nil,           // args
	)
	if err != nil {
		return false, fmt.Errorf("failed to set up response channel: %v", err)
	}

	responses := 0
	for {
		select {
		case response := <-responseChan:
			if response.RoutingKey == transactionID {
				
				var txnResponse TransactionResponse
				err := json.Unmarshal(response.Body, &txnResponse)
				if err != nil {
					return false, fmt.Errorf("failed to unmarshal response: %v", err)
				}

				slog.InfoContext(ctx, "received response", "transaction_id", transactionID, "is_valid", txnResponse.IsValid)

				if txnResponse.IsValid {
					responses++
				}
				if responses >= 5 {
					// Delete the queue after receiving 5 responses
					_, err := s.rabbitMQChan.QueueDelete(queueName, false, false, false)
					if err != nil {
						log.Printf("Failed to delete queue %s: %v", queueName, err)
					}
					return true, nil
				}
			}
		case <-ctx.Done():
			// Delete the queue after timeout
			_, err := s.rabbitMQChan.QueueDelete(queueName, false, false, false)
			if err != nil {
				log.Printf("Failed to delete queue %s: %v", queueName, err)
			}
			return false, nil
		}
	}
}
