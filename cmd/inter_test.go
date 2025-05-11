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
		t.Fatalf("Failed to create storage: %v", err)
	}
	if err := stor.Init(); err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	userID, err := stor.CreateUser("testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	token, err := auth.GenerateJWT(userID)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
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

	t.Run("Create expression", func(t *testing.T) {
		reqBody := []byte(`{"expression":"2+2*2"}`)
		req, err := http.NewRequest("POST", "http://localhost:8080/api/v1/calculate", bytes.NewReader(reqBody))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
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
			t.Errorf("Expected status 201, got %d", resp.StatusCode)
		}

		var calcResp struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&calcResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if calcResp.ID == "" {
			t.Error("Empty expression ID in response")
		}

		t.Run("Check expression status", func(t *testing.T) {
			var exprStatus string
			var result float64
			var lastErr error

			for i := 0; i < 10; i++ {
				time.Sleep(1 * time.Second)

				req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/expressions/"+calcResp.ID, nil)
				if err != nil {
					lastErr = err
					continue
				}
				req.Header.Set("Authorization", "Bearer "+token)

				resp, err := client.Do(req)
				if err != nil {
					lastErr = err
					continue
				}

				var exprResp struct {
					Expression struct {
						Status string  `json:"status"`
						Result float64 `json:"result"`
					} `json:"expression"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&exprResp); err != nil {
					resp.Body.Close()
					lastErr = err
					continue
				}
				resp.Body.Close()

				exprStatus = exprResp.Expression.Status
				result = exprResp.Expression.Result

				if exprStatus == "completed" || exprStatus == "error" {
					break
				}
			}

			if lastErr != nil {
				t.Fatalf("Last error during polling: %v", lastErr)
			}

			if exprStatus != "completed" {
				t.Errorf("Expression not completed, status: %s", exprStatus)
			}
			if result != 6 {
				t.Errorf("Incorrect result, got: %v, expected: 6", result)
			}
		})

		t.Run("Check expressions history", func(t *testing.T) {
			req, err := http.NewRequest("GET", "http://localhost:8080/api/v1/expressions", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Get expressions failed: %v", err)
			}
			defer resp.Body.Close()

			var exprsResp struct {
				Expressions []struct {
					ID     string  `json:"id"`
					Expr   string  `json:"expression"`
					Status string  `json:"status"`
					Result float64 `json:"result"`
				} `json:"expressions"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&exprsResp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if len(exprsResp.Expressions) == 0 {
				t.Error("No expressions found in history")
			}

			found := false
			for _, expr := range exprsResp.Expressions {
				if expr.ID == calcResp.ID {
					found = true
					if expr.Status != "completed" {
						t.Errorf("Expression status in history is %s, expected completed", expr.Status)
					}
					if expr.Result != 6 {
						t.Errorf("Expression result in history is %v, expected 6", expr.Result)
					}
					break
				}
			}

			if !found {
				t.Errorf("Created expression not found in history")
			}
		})
	})

	t.Run("Error handling", func(t *testing.T) {
		t.Run("Invalid token", func(t *testing.T) {
			reqBody := []byte(`{"expression":"2+2"}`)
			req, err := http.NewRequest("POST", "http://localhost:8080/api/v1/calculate", bytes.NewReader(reqBody))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer invalid_token")

			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("Expected status 401 for invalid token, got %d", resp.StatusCode)
			}
		})

		t.Run("Invalid expression", func(t *testing.T) {
			reqBody := []byte(`{"expression":"2++2"}`)
			req, err := http.NewRequest("POST", "http://localhost:8080/api/v1/calculate", bytes.NewReader(reqBody))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Errorf("Expected status 422 for invalid expression, got %d", resp.StatusCode)
			}
		})
	})
}