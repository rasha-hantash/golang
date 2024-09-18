package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
)

func main() {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://localhost:8080/concurrent", nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Read and print the "waitforquorum" message
	var quorumMessage map[string]string
	if err := json.NewDecoder(reader).Decode(&quorumMessage); err != nil {
		fmt.Printf("Error decoding quorum message: %v\n", err)
		return
	}
	fmt.Printf("Quorum message: %s\n", quorumMessage["message"])

	// Try to read more data (this should fail as the connection is closed)
	// _, err = reader.ReadByte()
	// if err != nil {
	// 	fmt.Printf("Connection closed after quorum message: %v\n", err)
	// } else {
	// 	fmt.Println("Unexpected: connection still open")
	// }

	fmt.Println("Client finished")
}