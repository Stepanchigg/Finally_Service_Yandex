package main

import (
	"log"

	"calc_service/internal/agent"
)

func main() {
	agent := agent.NewAgent()
	log.Println("Starting Agent...")
	agent.Start()
}
