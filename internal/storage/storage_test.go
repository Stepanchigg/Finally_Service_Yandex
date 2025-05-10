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
		t.Fatalf("не удалось создать хранилище: %v", err)
	}

	err = storage.Init()
	if err != nil {
		t.Fatalf("не удалось инициализировать хранилище: %v", err)
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
		t.Fatalf("GetUserByLogin не удалось: %v", err)
	}

	if user.ID != userID || user.Login != "testuser" {
		t.Errorf("не совпадают данные пользователя, имеем: %+v", user)
	}

	_, err = storage.CreateUser("testuser", "anotherpass")
	if err == nil {
		t.Error("ожидалась ошибка в дублировании пользователя, имеем nil")
	}
}

func TestExpressionOperations(t *testing.T) {
	storage := setupTestDB(t)

	userID, _ := storage.CreateUser("testuser", "hash")

	expr, err := storage.CreateExpression(userID, "2+2*2")
	if err != nil {
		t.Fatalf("CreateExpression не удалось: %v", err)
	}

	gotExpr, err := storage.GetExpressionByID(expr.ID, userID)
	if err != nil {
		t.Fatalf("GetExpressionByID не удалось: %v", err)
	}

	if gotExpr.Expression != "2+2*2" || gotExpr.Status != "pending" {
		t.Errorf("не совпадают данные выражения, имеем: %+v", gotExpr)
	}

	err = storage.UpdateExpression(&Expression{
		ID:     expr.ID,
		UserID: userID,
		Status: "completed",
		Result: func() *float64 { v := 6.0; return &v }(),
	})
	if err != nil {
		t.Fatalf("UpdateExpression не удалось: %v", err)
	}

	exprs, err := storage.GetExpressions(userID)
	if err != nil {
		t.Fatalf("GetExpressions не удалось: %v", err)
	}

	if len(exprs) != 1 || *exprs[0].Result != 6.0 {
		t.Errorf("GetExpressions вернула неожиданные данные: %+v", exprs)
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
		t.Fatalf("CreateTask не удалось: %v", err)
	}

	gotTask, err := storage.GetPendingTask()
	if err != nil {
		t.Fatalf("GetPendingTask не удалось: %v", err)
	}

	if gotTask.ID != "task1" || gotTask.Operation != "*" {
		t.Errorf("не совпадают данные задачи, имеем: %+v", gotTask)
	}

	err = storage.CompleteTask("task1", 4)
	if err != nil {
		t.Fatalf("CompleteTask не удалось: %v", err)
	}

	completedTask, err := storage.GetTaskByID("task1")
	if err != nil {
		t.Fatalf("GetTaskByID не удалось: %v", err)
	}

	if !completedTask.Completed || completedTask.Result.Float64 != 4 {
		t.Errorf("задача(Task) выполнилась недолжным образом, имеем: %+v", completedTask)
	}
}
