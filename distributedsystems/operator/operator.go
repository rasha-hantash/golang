package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/streadway/amqp"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

type Transaction struct {
	// Add fields as per your transaction structure
	ID string `json:"id"`
	// ... other fields
}

func main() {
	connectionString := "amqp://guest:guest@localhost:5672/"

	conn, err := amqp.Dial(connectionString)
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	// todo look more into this
	// err = ch.Qos(
	// 	1,     // prefetch count
	// 	0,     // prefetch size
	// 	false, // global
	// )
	// failOnError(err, "Failed to set QoS")

	transactionQueue := "transaction_requests"
	msgs, err := ch.Consume(
		"transaction_requests", // queue
		"",               // consumer
		false,            // auto-ack
		false,            // exclusive
		false,            // no-local
		false,            // no-wait
		nil,              // args
	)
	failOnError(err, "Failed to register a consumer")

	fmt.Printf("Operator is waiting for messages on queue: %s. To exit press CTRL+C\n", transactionQueue)

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			processTransaction(d, ch)
			d.Ack(false)
		}
	}()

	<-forever
}

func processTransaction(d amqp.Delivery, ch *amqp.Channel) {
	var tx Transaction
	err := json.Unmarshal(d.Body, &tx)
	if err != nil {
		log.Printf("Error decoding transaction: %v", err)
		return
	}

	// Simulate processing time
	time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

	// Simulate validation (replace with actual validation logic)
	isValid := rand.Float32() < 0.8 // 80% chance of being valid

	response := map[string]interface{}{
		"transaction_id": tx.ID,
		"is_valid":       isValid,
		// Add any other relevant response data
	}

	responseBody, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error encoding response: %v", err)
		return
	}

	// // Create a unique response queue for this transaction
	// _, err = s.rabbitMQChan.QueueDeclare(
	// 	d.CorrelationId,    // name (empty string means a random unique name will be generated)
	// 	false, // durable
	// 	false, // delete when unused
	// 	true,  // exclusive
	// 	false, // no-wait
	// 	nil,   // arguments
	// )
	// if err != nil {
	// 	// http.Error(w, "Failed to create response queue", http.StatusInternalServerError)
	// 	// todo modify this
	// 	return
	// }

	err = ch.Publish(
		"",        // exchange
		d.CorrelationId, // routing key (queue name)
		false,     // mandatory
		false,     // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			CorrelationId: d.CorrelationId,
			Body:          responseBody,
		})

	if err != nil {
		log.Printf("Error publishing response: %v", err)
	} else {
		log.Printf("Published response for transaction %s", tx.ID)
	}
}