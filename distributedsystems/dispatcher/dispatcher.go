package main

import (
	"context"
	"log"
	"net/http"
	"github.com/gorilla/mux"
	"golang.org/x/time/rate"
	"io"
	"encoding/json"
	"fmt"

	"time"

	"github.com/streadway/amqp"
	"github.com/segmentio/ksuid"
)

type TransactionRequest struct {
	To    string `json:"to"`
	From  string `json:"from"`
	Data  string `json:"data"`
	Value string `json:"value"`
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

type server struct {
	rabbitMQConn   *amqp.Connection
	rabbitMQChan   *amqp.Channel
	responseQueue  amqp.Queue
	transactionQueue string
}

func newServer(conn *amqp.Connection, ch *amqp.Channel, respQueue amqp.Queue, transQueue string) *server {
	return &server{
		rabbitMQConn:   conn,
		rabbitMQChan:   ch,
		responseQueue:  respQueue,
		transactionQueue: transQueue,
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

	// Declare the transactions queue
	transQueue, err := ch.QueueDeclare(
		"transaction_requests", // name
		false,                  // durable
		false,                  // delete when unused
		false,                  // exclusive
		false,                  // no-wait
		nil,                    // arguments
	)
	failOnError(err, "Failed to declare transactions queue")

	// Declare the response queue
	respQueue, err := ch.QueueDeclare(
		"transaction_responses",    // name (empty string means a random unique name will be generated)
		false, // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare response queue")

	// Create a new server instance
	s := newServer(conn, ch, respQueue, transQueue.Name)

	// Initialize and start the gateway
	router := mux.NewRouter()

	// Initialize rate limiter
	limiter := rate.NewLimiter(rate.Limit(2), 10) // 2 requests per second, allow bursts up to 10

	// Add rate limiter middleware
	router.Use(rateLimiterMiddleware(limiter))

	// Add health check endpoint
	router.HandleFunc("/health", healthCheckHandler).Methods("GET")

	// Add your routes here, for example:
	router.HandleFunc("/validate_transaction", s.ValidateTransaction).Methods("POST")

	// Start the HTTP server
	log.Fatal(http.ListenAndServe(":8080", router))
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

func (s *server) ValidateTransaction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the entire body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Check if the body is empty
	if len(body) == 0 {
		http.Error(w, "Request body is empty", http.StatusBadRequest)
		return
	}

	transactionID := fmt.Sprintf("txn_%s", ksuid.New().String())

	// Create a unique response queue for this transaction
	responseQueue, err := s.rabbitMQChan.QueueDeclare(
		transactionID,    // name (empty string means a random unique name will be generated)
		false, // durable
		false, // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		http.Error(w, "Failed to create response queue", http.StatusInternalServerError)
		return
	}

	fmt.Println(responseQueue.Name)

	// Publish the transaction to RabbitMQ
	err = s.rabbitMQChan.Publish(
		"fanout",                 // exchange
		s.transactionQueue, // routing key (queue name)
		false,              // mandatory
		false,              // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			CorrelationId: transactionID,
			// ReplyTo:       responseQueue.Name,
			Body:          body,
		})
	if err != nil {
		http.Error(w, "Failed to publish message", http.StatusInternalServerError)
		return
	}

	isCompliant, err := s.collectResponses(ctx, responseQueue.Name, transactionID)
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


func (s *server) collectResponses(ctx context.Context, queueName, correlationID string) (bool, error) {
	responseChan, err := s.rabbitMQChan.Consume(
		queueName, // queue
		"",        // consumer
		true,      // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		return false, fmt.Errorf("failed to set up response channel: %v", err)
	}

	responses := 0
	for {
		select {
		case response := <-responseChan:
			if response.CorrelationId == correlationID {
				responses++
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