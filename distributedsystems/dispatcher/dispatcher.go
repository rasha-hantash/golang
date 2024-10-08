package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"
	"io"
	"bytes"

	"github.com/gorilla/mux"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/segmentio/ksuid"
	"golang.org/x/time/rate"
	"github.com/spf13/viper"
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

type Server struct {
	rabbitMQConn *amqp.Connection
	rabbitMQChan *amqp.Channel
	router       *mux.Router
	limiter      *rate.Limiter
}

func NewServer(conn *amqp.Connection, ch *amqp.Channel) *Server {
	s := &Server{
		rabbitMQConn: conn,
		rabbitMQChan: ch,
		router:       mux.NewRouter(),
		limiter:      rate.NewLimiter(rate.Limit(2), 10),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.router.Use(s.rateLimiterMiddleware)
	s.router.HandleFunc("/health", s.healthCheckHandler).Methods("GET")
	s.router.HandleFunc("/transaction", s.BroadcastTransaction).Methods("POST")
}

func main() {
	viper.SetConfigFile("../.env")
    err := viper.ReadInConfig()
    if err != nil {
        fmt.Printf("Error reading config file: %s\n", err)
    }


	conn, ch := initRabbitMQ()
	defer conn.Close()
	defer ch.Close()

	server := NewServer(conn, ch)

	port := "8081"
	log.Printf("Server is now listening on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, server.router))
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

func initRabbitMQ() (*amqp.Connection, *amqp.Channel) {
	auth0Token, err := getAuth0Token()
	if err != nil {
		log.Fatalf("Error getting token: %v", err)
	}

	// RabbitMQ connection parameters
	rabbitmqURL := "amqp://localhost:5672" // Replace with your actual RabbitMQ host if different

	// Create a custom dialer that includes the OAuth2 token
	conn, err := amqp.DialConfig(rabbitmqURL, amqp.Config{
		Heartbeat: 10 * time.Second,
		Locale:    "en_US",
		SASL:      []amqp.Authentication{&amqp.PlainAuth{Username: "", Password: auth0Token}},
	})
	failOnError(err, "Failed to open a connection")
	fmt.Println("Successfully connected to RabbitMQ")
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

	return conn, ch
}

func (s *Server) BroadcastTransaction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()

	var txnRequest TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&txnRequest); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	txnID := fmt.Sprintf("txn_%s", ksuid.New().String())
	txnRequest.TransactionID = txnID

	responseQueue, err := s.createResponseQueue(txnID)
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to create response queue", http.StatusInternalServerError)
		return
	}

	if err := s.publishTransaction(ctx, txnRequest); err != nil {
		http.Error(w, "Failed to publish message", http.StatusInternalServerError)
		return
	}

	isCompliant, err := s.collectResponses(ctx, responseQueue.Name, txnID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONResponse(w, map[string]bool{"is_compliant": isCompliant})
}

func (s *Server) createResponseQueue(txnID string) (amqp.Queue, error) {
	return s.rabbitMQChan.QueueDeclare(
		txnID, // name
		false, // durable
		false, // auto delete
		true,  // exclusive -> important to ensure that no one else can recieve responses from this transaction responses queue 
		false,
		nil,
	)
}

func (s *Server) publishTransaction(ctx context.Context, txnRequest TransactionRequest) error {
	txnRequestByte, err := json.Marshal(txnRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction request: %w", err)
	}

	return s.rabbitMQChan.PublishWithContext(
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

func (s *Server) collectResponses(ctx context.Context, queueName, transactionID string) (bool, error) {
	slog.InfoContext(ctx, "collecting responses for transaction", "transaction_id", transactionID)
	responseChan, err := s.rabbitMQChan.Consume(
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
					s.deleteQueue(queueName)
					return true, nil
				}
			}
		case <-ctx.Done():
			s.deleteQueue(queueName)
			return false, nil
		}
	}
}

func (s *Server) deleteQueue(queueName string) {
	_, err := s.rabbitMQChan.QueueDelete(queueName, false, false, false)
	if err != nil {
		log.Printf("Failed to delete queue %s: %v", queueName, err)
	}
}

func (s *Server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) rateLimiterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.Allow() {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}