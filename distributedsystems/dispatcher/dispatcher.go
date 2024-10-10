package main

import (
	"context"
	"log"
	"net/http"
	"log/slog"
	"os"

	"github.com/gorilla/mux"
	"golang.org/x/time/rate"

	"github.com/rasha-hantash/golang/distributedsystems/dispatcher/config"
	"github.com/rasha-hantash/golang/distributedsystems/dispatcher/rabbitmq"
	"github.com/rasha-hantash/golang/distributedsystems/libs/auth"
	"github.com/rasha-hantash/golang/distributedsystems/libs/logger"
)



type Server struct {
	rmqSvc *rabbitmq.RabbitMQService
	router       *mux.Router
	limiter      *rate.Limiter
}



func main() {
	// load configuration 
	h := &logger.ContextHandler{Handler: slog.NewJSONHandler(os.Stdout, nil)}
	slog.SetDefault(slog.New(h))
    ctx := logger.AppendCtx(context.Background(), slog.String("request_id", "req-123"))

	cfg, err := config.LoadConfig(ctx)
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
	port := "80" // todo maybe put this in an env var? 
	slog.InfoContext(ctx, "server is now listening", slog.String("port", port))
	log.Fatal(http.ListenAndServe(":"+port, s.router)) // todo add host no? 
}



func NewServer(rabbitmqSvc *rabbitmq.RabbitMQService) *Server {
	s := &Server{
		rmqSvc: rabbitmqSvc,
		router:       mux.NewRouter(),
		limiter:      rate.NewLimiter(rate.Limit(2), 10),
	}

	s.router.Use(s.rateLimiterMiddleware)
	s.router.HandleFunc("/health", s.healthCheckHandler).Methods("GET")
	s.router.HandleFunc("/transaction", s.rmqSvc.BroadcastTransaction).Methods("POST")
	return s
}

func (s *Server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	slog.InfoContext(r.Context(), "health check")
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


