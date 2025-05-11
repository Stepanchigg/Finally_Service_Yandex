package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"calc_service/internal/agent"
	"calc_service/internal/auth"
	"calc_service/internal/orchestrator"
	"calc_service/internal/storage"
)

func setupTestEnvironment(t *testing.T) (func(), string) {
	dbPath := "test_integration.db"
	_ = os.Remove(dbPath)

	stor, err := storage.NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Не удалось создать storage: %v", err)
	}
	if err := stor.Init(); err != nil {
		t.Fatalf("Не удалось инициализировать storage: %v", err)
	}

	userID, err := stor.CreateUser("testuser", "testpass")
	if err != nil {
		t.Fatalf("Не удалось создать test user: %v", err)
	}

	token, err := auth.GenerateJWT(userID)
	if err != nil {
		t.Fatalf("Не удалось сгенерировать token: %v", err)
	}

	orch := orchestrator.NewOrchestrator()
	orch.Storage = stor
	go func() {
		if err := orch.RunServer(); err != nil && err != http.ErrServerClosed {
			t.Logf("Orchestrator failed: %v", err)
		}
	}()

	ag := agent.NewAgent()
	ag.OrchestratorURL = "localhost:50051"
	go ag.Start()

	time.Sleep(2 * time.Second)

	return func() {
		stor.GetDB().Close()
		_ = os.Remove(dbPath)
	}, token
}

func TestFullWorkflow(t *testing.T) {
	cleanup, token := setupTestEnvironment(t)
	defer cleanup()

	reqBody := []byte(`{"expression":"2+2*2"}`)
	req, err := http.NewRequest("POST", "http://localhost:8080/api/v1/calculate", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("Не удалось создать request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Calculate request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Ожидалось status 201, got %d", resp.StatusCode)
	}

	var calcResp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&calcResp); err != nil {
		t.Fatalf("Не удалось декодировать response: %v", err)
	}
	if calcResp.ID == "" {
		t.Error("Пустое ID выражения in response")
	}

	var exprStatus string
	var result float64
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)

		req, err = http.NewRequest("GET", "http://localhost:8080/api/v1/expressions/"+calcResp.ID, nil)
		if err != nil {
			t.Fatalf("Не удалось создать request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err = client.Do(req)
		if err != nil {
			t.Fatalf("Не удалось получить выражение: %v", err)
		}

		var exprResp struct {
			Expression struct {
				Status string  `json:"status"`
				Result float64 `json:"result"`
			} `json:"expression"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&exprResp); err != nil {
			resp.Body.Close()
			t.Fatalf("Не удалось декодировать response: %v", err)
		}
		resp.Body.Close()

		exprStatus = exprResp.Expression.Status
		result = exprResp.Expression.Result

		if exprStatus == "completed" {
			break
		}
	}

	if exprStatus != "completed" {
		t.Errorf("Выражение не завершено, status: %s", exprStatus)
	}
	if result != 6 {
		t.Errorf("Неверный результат, имеем: %v, Ожидалось: 6", result)
	}

	req, err = http.NewRequest("GET", "http://localhost:8080/api/v1/expressions", nil)
	if err != nil {
		t.Fatalf("Не удалось создать запрос: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("Не удалось получить выражения: %v", err)
	}
	defer resp.Body.Close()

	var exprsResp struct {
		Expressions []struct {
			ID     string  `json:"id"`
			Result float64 `json:"result"`
		} `json:"expressions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&exprsResp); err != nil {
		t.Fatalf("Не удалось декодировать response: %v", err)
	}

	if len(exprsResp.Expressions) != 1 || exprsResp.Expressions[0].Result != 6 {
		t.Errorf("Неожиданный expressions list: %+v", exprsResp)
	}
}
