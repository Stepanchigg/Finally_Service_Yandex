package storage

import (
	"os"
	"testing"
)

func setupTestDB(t *testing.T) *Storage {
	dbPath := "test_db.sqlite"
	os.Remove(dbPath)

	storage, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	err = storage.Init()
	if err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	t.Cleanup(func() {
		storage.GetDB().Close()
		os.Remove(dbPath)
	})

	return storage
}

func TestUserOperations(t *testing.T) {
	storage := setupTestDB(t)

	userID, err := storage.CreateUser("testuser", "hashedpassword")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	user, err := storage.GetUserByLogin("testuser")
	if err != nil {
		t.Fatalf("GetUserByLogin failed: %v", err)
	}

	if user.ID != userID || user.Login != "testuser" {
		t.Errorf("User data mismatch, got: %+v", user)
	}

	_, err = storage.CreateUser("testuser", "anotherpass")
	if err == nil {
		t.Error("Expected error for duplicate user, got nil")
	}
}

func TestExpressionOperations(t *testing.T) {
	storage := setupTestDB(t)

	userID, _ := storage.CreateUser("testuser", "hash")

	expr, err := storage.CreateExpression(userID, "2+2*2")
	if err != nil {
		t.Fatalf("CreateExpression failed: %v", err)
	}

	gotExpr, err := storage.GetExpressionByID(expr.ID, userID)
	if err != nil {
		t.Fatalf("GetExpressionByID failed: %v", err)
	}

	if gotExpr.Expression != "2+2*2" || gotExpr.Status != "pending" {
		t.Errorf("Expression data mismatch, got: %+v", gotExpr)
	}

	err = storage.UpdateExpression(&Expression{
		ID:     expr.ID,
		UserID: userID,
		Status: "completed",
		Result: func() *float64 { v := 6.0; return &v }(),
	})
	if err != nil {
		t.Fatalf("UpdateExpression failed: %v", err)
	}

	exprs, err := storage.GetExpressions(userID)
	if err != nil {
		t.Fatalf("GetExpressions failed: %v", err)
	}

	if len(exprs) != 1 || *exprs[0].Result != 6.0 {
		t.Errorf("GetExpressions returned unexpected data: %+v", exprs)
	}
}

func TestTaskOperations(t *testing.T) {
	storage := setupTestDB(t)

	userID, _ := storage.CreateUser("testuser", "hash")
	expr, _ := storage.CreateExpression(userID, "2+2*2")

	task := &Task{
		ID:            "task1",
		ExprID:        expr.ID,
		Arg1:          2,
		Arg2:          2,
		Operation:     "*",
		OperationTime: 100,
	}
	err := storage.CreateTask(task)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	gotTask, err := storage.GetPendingTask()
	if err != nil {
		t.Fatalf("GetPendingTask failed: %v", err)
	}

	if gotTask.ID != "task1" || gotTask.Operation != "*" {
		t.Errorf("Task data mismatch, got: %+v", gotTask)
	}

	err = storage.CompleteTask("task1", 4)
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	completedTask, err := storage.GetTaskByID("task1")
	if err != nil {
		t.Fatalf("GetTaskByID failed: %v", err)
	}

	if !completedTask.Completed || completedTask.Result.Float64 != 4 {
		t.Errorf("Task not completed properly, got: %+v", completedTask)
	}
}
