package storage

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"math"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

var embedMigrations embed.FS

type User struct {
	ID       int
	Login    string
	Password string
}

type Expression struct {
	ID         int
	UserID     int
	Expression string
	Status     string
	Result     *float64
	CreatedAt  time.Time
}

type Task struct {
	ID            string
	ExprID        int
	Arg1          float64
	Arg2          float64
	Operation     string
	OperationTime int
	StartedAt     sql.NullTime
	Completed     bool
	Result        sql.NullFloat64
}

type Storage struct {
	db *sql.DB
}

func (s *Storage) GetDB() *sql.DB {
	return s.db
}

func (s *Storage) CreateUser(login, password string) (int, error) {
	var id int
	err := s.db.QueryRow(
		"INSERT INTO users (login, password) VALUES (?, ?) RETURNING id",
		login, password,
	).Scan(&id)

	if err != nil {
		if isDuplicate(err) {
			return 0, ErrAlreadyExists
		}
		return 0, fmt.Errorf("create user: %w", err)
	}
	return id, nil
}

func (s *Storage) GetUserByLogin(login string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		"SELECT id, login, password FROM users WHERE login = ?",
		login,
	).Scan(&u.ID, &u.Login, &u.Password)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func (s *Storage) GetUserByID(id int) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		"SELECT id, login, password FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Login, &u.Password)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

func (s *Storage) DeleteUser(id int) error {
	_, err := s.db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (s *Storage) CreateExpression(userID int, expr string) (*Expression, error) {
	e := &Expression{
		UserID:     userID,
		Expression: expr,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	err := s.db.QueryRow(
		`INSERT INTO expressions 
		(user_id, expression, status, created_at) 
		VALUES (?, ?, ?, ?) 
		RETURNING id`,
		e.UserID, e.Expression, e.Status, e.CreatedAt,
	).Scan(&e.ID)

	if err != nil {
		return nil, fmt.Errorf("create expression: %w", err)
	}
	return e, nil
}

func (s *Storage) GetExpressionByID(id, userID int) (*Expression, error) {
	e := &Expression{ID: id, UserID: userID}
	var result sql.NullFloat64
	err := s.db.QueryRow(
		`SELECT expression, status, result, created_at 
		FROM expressions 
		WHERE id = ? AND user_id = ?`,
		id, userID,
	).Scan(&e.Expression, &e.Status, &result, &e.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get expression: %w", err)
	}

	if result.Valid {
		e.Result = &result.Float64
	}
	return e, nil
}

func (s *Storage) GetExpressions(userID int) ([]*Expression, error) {
	rows, err := s.db.Query(
		`SELECT id, expression, status, result 
         FROM expressions 
         WHERE user_id = ? 
         ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exprs []*Expression
	for rows.Next() {
		e := &Expression{}
		var result sql.NullFloat64
		err := rows.Scan(&e.ID, &e.Expression, &e.Status, &result)
		if err != nil {
			return nil, err
		}
		if result.Valid {
			e.Result = &result.Float64
		}
		exprs = append(exprs, e)
	}
	return exprs, nil
}

func (s *Storage) UpdateExpression(e *Expression) error {
	var result interface{}
	if e.Result != nil {
		result = *e.Result
	}

	_, err := s.db.Exec(
		`UPDATE expressions 
		SET status = ?, result = ? 
		WHERE id = ? AND user_id = ?`,
		e.Status, result, e.ID, e.UserID,
	)
	return err
}

func (s *Storage) DeleteExpression(id, userID int) error {
	_, err := s.db.Exec(
		"DELETE FROM expressions WHERE id = ? AND user_id = ?",
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("delete expression: %w", err)
	}
	return nil
}

func (s *Storage) CreateTask(t *Task) error {
	_, err := s.db.Exec(
		`INSERT INTO tasks 
        (id, expression_id, arg1, arg2, operation, operation_time) 
        VALUES (?, ?, ?, ?, ?, ?)`,
		t.ID, t.ExprID, t.Arg1, t.Arg2, t.Operation, t.OperationTime,
	)
	return err
}

func (s *Storage) GetPendingTask() (*Task, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	t := &Task{}
	err = tx.QueryRow(
		`SELECT id, expression_id, arg1, arg2, operation, operation_time 
         FROM tasks 
         WHERE completed = FALSE 
         ORDER BY id ASC 
         LIMIT 1`).Scan(
		&t.ID, &t.ExprID, &t.Arg1, &t.Arg2, &t.Operation, &t.OperationTime,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	_, err = tx.Exec(
		`UPDATE tasks SET started_at = datetime('now') WHERE id = ?`,
		t.ID,
	)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	return t, err
}

func (s *Storage) GetTaskByID(id string) (*Task, error) {
	t := &Task{}
	err := s.db.QueryRow(
		`SELECT id, expression_id, arg1, arg2, operation, operation_time, 
		started_at, completed, result 
		FROM tasks WHERE id = ?`,
		id,
	).Scan(
		&t.ID, &t.ExprID, &t.Arg1, &t.Arg2, &t.Operation, &t.OperationTime,
		&t.StartedAt, &t.Completed, &t.Result,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get task: %w", err)
	}
	return t, nil
}

func (s *Storage) GetTasksByExpressionID(exprID int) ([]*Task, error) {
	rows, err := s.db.Query(
		`SELECT id, arg1, arg2, operation, operation_time, 
		started_at, completed, result 
		FROM tasks WHERE expression_id = ?`,
		exprID,
	)
	if err != nil {
		return nil, fmt.Errorf("get tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t := &Task{ExprID: exprID}
		err := rows.Scan(
			&t.ID, &t.Arg1, &t.Arg2, &t.Operation, &t.OperationTime,
			&t.StartedAt, &t.Completed, &t.Result,
		)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *Storage) CompleteTask(taskID string, result float64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var exprID int
	err = tx.QueryRow(
		`UPDATE tasks 
         SET completed = TRUE, result = ?
         WHERE id = ? 
         RETURNING expression_id`,
		result, taskID,
	).Scan(&exprID)
	if err != nil {
		return fmt.Errorf("failed to update task: %v", err)
	}

	var pendingCount int
	err = tx.QueryRow(
		`SELECT COUNT(*) FROM tasks 
         WHERE expression_id = ? AND completed = FALSE`,
		exprID,
	).Scan(&pendingCount)
	if err != nil {
		return fmt.Errorf("failed to check pending tasks: %v", err)
	}

	if pendingCount == 0 {
		rows, err := tx.Query(
			`SELECT operation, result FROM tasks 
             WHERE expression_id = ? ORDER BY id`,
			exprID,
		)
		if err != nil {
			return fmt.Errorf("failed to get task results: %v", err)
		}
		defer rows.Close()

		var finalResult float64
		var hasError bool

		for rows.Next() {
			var op string
			var res float64
			if err := rows.Scan(&op, &res); err != nil {
				hasError = true
				break
			}

			if op == "/" && res == math.Inf(1) {
				hasError = true
				break
			}
		}

		if hasError {
			_, err = tx.Exec(
				`UPDATE expressions 
                 SET status = 'error'
                 WHERE id = ?`,
				exprID,
			)
		} else {
			err = tx.QueryRow(
				`SELECT SUM(result) FROM tasks 
                 WHERE expression_id = ?`,
				exprID,
			).Scan(&finalResult)
			if err != nil {
				return fmt.Errorf("failed to calculate final result: %v", err)
			}

			_, err = tx.Exec(
				`UPDATE expressions 
                 SET status = 'completed', result = ?
                 WHERE id = ?`,
				finalResult, exprID,
			)
		}

		if err != nil {
			return fmt.Errorf("failed to update expression: %v", err)
		}
	}

	return tx.Commit()
}
func (s *Storage) GetPendingTasksCount() (int, error) {
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM tasks WHERE completed = FALSE",
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get pending tasks count: %w", err)
	}
	return count, nil
}

func (s *Storage) GetCompletedTasksCount() (int, error) {
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM tasks WHERE completed = TRUE",
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get completed tasks count: %w", err)
	}
	return count, nil
}

func isDuplicate(err error) bool {
	return err != nil && err.Error() == "UNIQUE constraint failed: users.login"
}

func NewStorage(dbPath string) (*Storage, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	storage := &Storage{db: db}
	if err := storage.Init(); err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	return storage, nil
}

func (s *Storage) Init() error {
	_, err := s.db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            login TEXT NOT NULL UNIQUE,
            password TEXT NOT NULL
        );

        CREATE TABLE IF NOT EXISTS expressions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            expression TEXT NOT NULL,
            status TEXT NOT NULL,
            result REAL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(user_id) REFERENCES users(id)
        );

        CREATE TABLE IF NOT EXISTS tasks (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            expression_id INTEGER NOT NULL,
            arg1 REAL NOT NULL,
            arg2 REAL NOT NULL,
            operation TEXT NOT NULL,
            operation_time INTEGER NOT NULL,
            started_at DATETIME,
            completed BOOLEAN DEFAULT FALSE,
            result REAL,
            FOREIGN KEY(expression_id) REFERENCES expressions(id)
        );
    `)
	return err
}
