package main

import (
	"log"

	"calc_service/internal/agent"
)

func main() {
	agent := agent.NewAgent()
	log.Println("Запусаем Agent...")
	agent.Start()
}
