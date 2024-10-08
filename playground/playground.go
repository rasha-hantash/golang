package main 

import (
	"encoding/json"
	"fmt"
	"os"
)

type Shift struct {
	ID             string    `json:"id"`
	CompanyID      string    `json:"companyId"`
	Slots          int 	     `json:"slots"`
}


type Signups struct {
	WorkerId string  `json:"workerId"`
	ShiftId string  `json:"shiftd"`
	Status string `json:"status"`
}

type Worker struct {
	ID           string         `json:"id"`
}


func main() {
	// Read JSON file
	jsonBytes, err := os.ReadFile("data.json")
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}

	// Parse JSON data
	var workers Worker
	err = json.Unmarshal(jsonBytes, &workers)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	// get shift 

	

	// if shift.max != nil 
		// determine if worker is new 
		// determine if 


	// else 
		// // get # of slots filled 

// 		- raised seed round end of last year in november 

// 		- june beta launch 
// 		- now want to launch roll out in full 
// 		- backend focused 
// 		- vanderbilt in 2021 (cs/math) -> software engineer in coinbase -> execution services 
// 		- coinbase prime wasn't built yet 
// 		-> brand new trading engine 
		
// 		- launch out of beta and onboard a lot more customers
// 		- initial pilot customers
// 		- warm intros with investors and networks
// 		-> manual to fully automated system for trading 
// 		-> full robust e2e hedging solutoin 
// 		-> scaling 
// 		-> adding more prudcts and algos for those products 
// 		-> adding support for more and more products 
// 			-> modular 
// 			-> ingesting market data form a new source 
// 			-> strategies are different 
// 			-> adding new features (payment solution)
// 			-> give us their data -> being able to parse any spreadsheets 
// 			-> automating other parts of that including reporting 
// }

// func canFillShift(workers []*Worker, shift Shift, workerID string) (bool, *Worker) {
// 	isAvailable := false 
// 	for _, worker := range workers {
// 		for _, availability := range worker.Availability{
// 			if availability.Date == shift.Date && !shift.StartTime.Before(availability.Start) && !shift.EndTime.After(availability.End) {
// 				isAvailable = true 
// 				return isAvailable, worker
// 			}
// 		}
// 	}

// 	return isAvailable, nil
}