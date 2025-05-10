package main

import (
	"log"

	"github/stepanchigg/Final_Zad_2_Vozvrat/internal/agent"
)

func main() {
	agent := agent.NewAgent()
	log.Println("Starting Agent...")
	agent.Start()
}