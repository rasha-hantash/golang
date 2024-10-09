package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"golang.org/x/time/rate"

	"github.com/rasha-hantash/golang/distributedsystems/libs/auth"
	"github.com/rasha-hantash/golang/distributedsystems/dispatcher/config"
	"github.com/rasha-hantash/golang/distributedsystems/dispatcher/rabbitmq"
)



type Server struct {
	rmqSvc *rabbitmq.RabbitMQService
	router       *mux.Router
	limiter      *rate.Limiter
}



func main() {
	// load configuration 
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize RabbitMQ connection
	rabbitCfg := rabbitmq.RabbitMQConfig{
		Host: cfg.RabbitMQHost,
		Port: "5672", // Assuming default port, adjust if needed
		Auth0Config: auth.Auth0Config{
			Domain:       cfg.Auth0Domain,
			ClientID:     cfg.DispatcherClientID,
			ClientSecret: cfg.DispatcherClientSecret,
			Audience:     "rabbitmq",
		},
	}
	rabbitmqSvc, err := rabbitmq.NewConnection(rabbitCfg)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitmqSvc.Close()

	s := NewServer(rabbitmqSvc)

	router := mux.NewRouter()
	router.Use(s.rateLimiterMiddleware)
	router.HandleFunc("/health", s.healthCheckHandler).Methods("GET")
	router.HandleFunc("/transaction", s.rmqSvc.BroadcastTransaction).Methods("POST")
	port := "8081" // todo maybe put this in an env var? 
	log.Printf("Server is now listening on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, s.router))
}



func NewServer(rabbitmqSvc *rabbitmq.RabbitMQService) *Server {
	s := &Server{
		rmqSvc: rabbitmqSvc,
		// rabbitMQConn: conn,
		// rabbitMQChan: ch,
		router:       mux.NewRouter(),
		limiter:      rate.NewLimiter(rate.Limit(2), 10),
	}
	return s
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


