package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)
//глобальные ошибки для операций
var (
	ErrDivisionByZero  = errors.New("деление на ноль")
	ErrInvalidOperator = errors.New("невалидный оператор")
)
//Agent представляет вычислительный агент с указанием мощности и адреса оркестратора
type Agent struct {
	ComputingPower  int
	OrchestratorURL string
	Con *grpc.ClientConn
	Client proto.CalcClient
}
//новый агент
func NewAgent() *Agent {
	cp, err := strconv.Atoi(os.Getenv("COMPUTING_POWER"))
	if err != nil || cp < 1 {
		cp = 1
	}

	orchestratorURL := os.Getenv("ORCHESTRATOR_URL")

	if orchestratorURL == "" {
		orchestratorURL = "http://localhost:8080"
	}
	return &Agent{
		ComputingPower:  cp,
		OrchestratorURL: orchestratorURL,
	}
}
//запускает указанное количество воркеров и блокирует выполнение
func (a *Agent) Start() {
	for i := 0; i < a.ComputingPower; i++ {
		log.Printf("Стуртующий воркер %d", i)
		go a.Worker(i)
	}
	select {}
}

func (a *Agent) Worker(id int) {
	for {
		resp, err := http.Get(a.OrchestratorURL + "/internal/task")
		if err != nil {
			log.Printf("Воркер %d: ошибка в задаче: %v", id, err)
			time.Sleep(2 * time.Second)
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			time.Sleep(1 * time.Second)
			continue
		}

		var taskResp struct {
			Task struct {
				ID            string  `json:"id"`
				Arg1          float64 `json:"arg1"`
				Arg2          float64 `json:"arg2"`
				Operation     string  `json:"operation"`
				OperationTime int     `json:"operation_time"`
			} `json:"task"`
		}

		err = json.NewDecoder(resp.Body).Decode(&taskResp)
		resp.Body.Close()
		if err != nil {
			log.Printf("Воркер %d: Ошибка в декодировании задачи: %v", id, err)
			time.Sleep(1 * time.Second)
			continue
		}

		task := taskResp.Task
		log.Printf("Воркер %d: полученное задание %s: %f %s %f, симулирует %d мс", id, task.ID, task.Arg1, task.Operation, task.Arg2, task.OperationTime)
		time.Sleep(time.Duration(task.OperationTime) * time.Millisecond)
		result, err := Calculations(task.Operation, task.Arg1, task.Arg2)
		if err != nil {
			log.Printf("Воркер %d: Ошибка в вычислении задачи %s: %v", id, task.ID, err)
			continue
		}

		resultPayload := map[string]interface{}{
			"id":     task.ID,
			"result": result,
		}

		payloadBytes, _ := json.Marshal(resultPayload)
		respPost, err := http.Post(a.OrchestratorURL+"/internal/task", "application/json", bytes.NewReader(payloadBytes))

		if err != nil {
			log.Printf("Воркер %d: Ошибка в записи результата задачи %s: %v", id, task.ID, err)
			continue
		}

		if respPost.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(respPost.Body)
			log.Printf("Воркер %d: Ошибка в записи результата задачи %s: %s", id, task.ID, string(body))
		} else {
			log.Printf("Воркер %d: Успешное выполненеи задачи %s с результатом %f", id, task.ID, result)
		}
		respPost.Body.Close()
	}
}
//заглушка для будущей реализации вычисления выражений
func CalculateExpression(expression string) (float64, error) {
	return 0, fmt.Errorf("not implemented")
}

func Calculations(operation string, a, b float64) (float64, error) {
	switch operation {
	case "+":
		return a + b, nil
	case "-":
		return a - b, nil
	case "*":
		return a * b, nil
	case "/":
		if b == 0 {
			return 0, ErrDivisionByZero
		}
		return a / b, nil
	default:
		return 0, fmt.Errorf("invalid operator: %s", operation)
	}
}