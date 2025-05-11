package main

import (
	"log"

	"calc_service/internal/orchestrator"
)

func main() {
	app := orchestrator.NewOrchestrator()
	log.Println("Запускаем Orchestrator на порту", app.Config.HTTPAddr)
	if err := app.RunServer(); err != nil {
		log.Fatal(err)
	}
}
