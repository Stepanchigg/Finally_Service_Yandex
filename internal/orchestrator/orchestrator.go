package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"google.golang.org/grpc"
	"calc_service/internal/authy"
	"calc_service/internal/proto"
	"calc_service/internal/storage"
)

type server struct {
	proto.UnimplementedCalculatorServer
	o *Orchestrator
}

type Config struct {
	HTTPAddr            string
	GRPCAddr            string
	TimeAddition        int
	TimeSubtraction     int
	TimeMultiplications int
	TimeDivisions       int
}

type Orchestrator struct {
	Config      *Config
	exprStore   map[string]*Expression
	taskStore   map[string]*Task
	taskQueue   []*Task
	mu          sync.Mutex
	exprCounter int64
	taskCounter int64
	Storage     *storage.Storage
}

type Expression struct {
	ID     string   `json:"id"`
	Expr   string   `json:"expression"`
	Status string   `json:"status"`
	Result *float64 `json:"result,omitempty"`
	AST    *ASTNode `json:"-"`
}

type Task struct {
	ID            string   `json:"id"`
	ExprID        string   `json:"-"`
	Arg1          float64  `json:"arg1"`
	Arg2          float64  `json:"arg2"`
	Operation     string   `json:"operation"`
	OperationTime int      `json:"operation_time"`
	Node          *ASTNode `json:"-"`
}

func Configuration() *Config {
	httpPort := os.Getenv("HTTP_PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50051"
	}

	ta, _ := strconv.Atoi(os.Getenv("TIME_ADDITION_MS"))
	if ta == 0 {
		ta = 100
	}

	ts, _ := strconv.Atoi(os.Getenv("TIME_SUBTRACTION_MS"))
	if ts == 0 {
		ts = 100
	}

	tm, _ := strconv.Atoi(os.Getenv("TIME_MULTIPLICATIONS_MS"))
	if tm == 0 {
		tm = 100
	}

	td, _ := strconv.Atoi(os.Getenv("TIME_DIVISIONS_MS"))
	if td == 0 {
		td = 100
	}

	return &Config{
		HTTPAddr:            httpPort,
		GRPCAddr:            grpcPort,
		TimeAddition:        ta,
		TimeSubtraction:     ts,
		TimeMultiplications: tm,
		TimeDivisions:       td,
	}
}

func (s *server) GetTask(ctx context.Context, req *proto.TaskRequest) (*proto.TaskResponse, error) {
	task, err := s.o.Storage.GetPendingTask()
	if err != nil {
		return nil, err
	}

	return &proto.TaskResponse{
		Id:            task.ID,
		Arg1:          task.Arg1,
		Arg2:          task.Arg2,
		Operation:     task.Operation,
		OperationTime: int32(task.OperationTime),
	}, nil
}

func (s *server) SubmitResult(ctx context.Context, req *proto.ResultRequest) (*proto.ResultResponse, error) {
	if err := s.o.Storage.CompleteTask(req.Id, req.Result); err != nil {
		return nil, err
	}
	return &proto.ResultResponse{Success: true}, nil
}

func NewOrchestrator() *Orchestrator {
	storage, err := storage.NewStorage("calc_service.db")
	if err != nil {
		log.Fatal(err)
	}

	err = storage.Init()
	if err != nil {
		log.Fatal(err)
	}

	return &Orchestrator{
		Config:    Configuration(),
		Storage:   storage,
		exprStore: make(map[string]*Expression),
		taskStore: make(map[string]*Task),
		taskQueue: make([]*Task, 0),
	}
}

func (o *Orchestrator) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"недоступный метод"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"невалидное тело запроса"}`, http.StatusBadRequest)
		return
	}

	if req.Login == "" || req.Password == "" {
		http.Error(w, `{"error":"Требуются логин и пароль"}`, http.StatusBadRequest)
		return
	}

	hashedPassword, err := authy.GenerateHash(req.Password)
	if err != nil {
		log.Printf("Failed to hash password: %v", err)
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		return
	}

	userID, err := o.Storage.CreateUser(req.Login, hashedPassword)
	if err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
			http.Error(w, `{"error":"Пользователь уже существует"}`, http.StatusConflict)
			return
		}
		log.Printf("Failed to create user: %v", err)
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":    userID,
		"login": req.Login,
	})
}

func (o *Orchestrator) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"недоступный метод"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"невалидное тело запроса"}`, http.StatusBadRequest)
		return
	}

	user, err := o.Storage.GetUserByLogin(req.Login)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, `{"error":"не найден пользователь"}`, http.StatusUnauthorized)
			return
		}
		log.Printf("Failed to get user: %v", err)
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		return
	}

	if err := authy.CompareHash(req.Password, user.Password); err!=nil {
		http.Error(w, `{"error":"неверный пароль"}`, http.StatusUnauthorized)
		return
	}

	token, err := authy.GenerateJWT(user.ID)
	if err != nil {
		log.Printf("проблема в генерации токена: %v", err)
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":    user.ID,
			"login": user.Login,
		},
	})
}

func (o *Orchestrator) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Incoming request to: %s", r.URL.Path)

		if r.URL.Path == "/login" || r.URL.Path == "/register" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"Требуется заголовок авторизации"}`, http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == "" {
			http.Error(w, `{"error":"неверный заголовок авторизации"}`, http.StatusUnauthorized)
			return
		}

		userID, err := authy.ParseJWT(tokenString)
		if err != nil {
			http.Error(w, `{"error":"неверный токен"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "userID", userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (o *Orchestrator) calculateHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Получен запрос на расчет")
	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		http.Error(w, `{"error":"неавторизован"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Expression string `json:"expression"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"неверное body"}`, http.StatusUnprocessableEntity)
		return
	}

	dbExpr, err := o.Storage.CreateExpression(userID, req.Expression)
	if err != nil {
		http.Error(w, `{"error":"проблема в создании выражения"}`, http.StatusInternalServerError)
		return
	}

	expr := &Expression{
		ID:     strconv.Itoa(dbExpr.ID),
		Expr:   req.Expression,
		Status: "pending",
	}

	ast, err := ParseAST(req.Expression)
	if err != nil {
		o.Storage.UpdateExpression(&storage.Expression{
			ID:     dbExpr.ID,
			Status: "error",
		})
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusUnprocessableEntity)
		return
	}

	expr.AST = ast
	o.Tasks(expr)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": expr.ID})
}

func (o *Orchestrator) expressionsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		http.Error(w, `{"error":"неавторизован"}`, http.StatusUnauthorized)
		return
	}

	dbExprs, err := o.Storage.GetExpressions(userID)
	if err != nil {
		http.Error(w, `{"error":"проблема в получении выражения"}`, http.StatusInternalServerError)
		return
	}

	response := make([]map[string]interface{}, len(dbExprs))
	for i, expr := range dbExprs {
		item := map[string]interface{}{
			"id":         strconv.Itoa(expr.ID),
			"expression": expr.Expression,
			"status":     expr.Status,
		}
		if expr.Result != nil {
			item["result"] = *expr.Result
		}
		response[i] = item
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"expressions": response})
}

func (o *Orchestrator) expressionIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"неверный метод"}`, http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value("userID").(int)
	if !ok {
		http.Error(w, `{"error":"неавторизован"}`, http.StatusUnauthorized)
		return
	}

	idStr := r.URL.Path[len("/api/v1/expressions/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, `{"error":"невалидное ID выражения"}`, http.StatusBadRequest)
		return
	}

	dbExpr, err := o.Storage.GetExpressionByID(id, userID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, `{"error":"выражение не найдено"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"проблема в получнии выражения"}`, http.StatusInternalServerError)
		return
	}

	expr := &Expression{
		ID:     idStr,
		Expr:   dbExpr.Expression,
		Status: dbExpr.Status,
		Result: dbExpr.Result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"expression": expr})

	response := map[string]interface{}{
		"id":         idStr,
		"expression": dbExpr.Expression,
		"status":     dbExpr.Status,
	}
	if dbExpr.Result != nil {
		response["result"] = *dbExpr.Result
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"expression": response})
}

func (o *Orchestrator) getTaskHandler(w http.ResponseWriter, r *http.Request) {
	task, err := o.Storage.GetPendingTask()
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.Error(w, `{"error":"нет доступной задачи"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error":"Internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"task": task})
}

func (o *Orchestrator) postTaskHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string  `json:"id"`
		Result float64 `json:"result"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"невалидное тело"}`, http.StatusUnprocessableEntity)
		return
	}

	if err := o.Storage.CompleteTask(req.ID, req.Result); err != nil {
		http.Error(w, `{"error":"проблема с завершением задачи"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"result accepted"}`))
}

func (o *Orchestrator) Tasks(expr *Expression) {
	log.Printf("создаем задачи из выражения %s", expr.ID)
	exprID, _ := strconv.Atoi(expr.ID)

	var stack []*ASTNode
	var tasks []*Task

	var postOrder func(node *ASTNode)
	postOrder = func(node *ASTNode) {
		if node == nil {
			return
		}

		postOrder(node.Left)
		postOrder(node.Right)

		if !node.IsLeaf {
			if len(stack) < 2 {
				log.Printf("Недостаточно операндов для работы %s", node.Operator)
				return
			}

			right := stack[len(stack)-1]
			left := stack[len(stack)-2]
			stack = stack[:len(stack)-2]

			o.taskCounter++
			taskID := fmt.Sprintf("%d", o.taskCounter)

			var opTime int
			switch node.Operator {
			case "+":
				opTime = o.Config.TimeAddition
			case "-":
				opTime = o.Config.TimeSubtraction
			case "*":
				opTime = o.Config.TimeMultiplications
			case "/":
				opTime = o.Config.TimeDivisions
			default:
				opTime = 100
			}

			task := &Task{
				ID:            taskID,
				ExprID:        expr.ID,
				Arg1:          left.Value,
				Arg2:          right.Value,
				Operation:     node.Operator,
				OperationTime: opTime,
				Node:          node,
			}

			tasks = append(tasks, task)

			resultNode := &ASTNode{
				IsLeaf: true,
				Value:  0,
			}
			stack = append(stack, resultNode)
		} else {
			stack = append(stack, node)
		}
	}

	postOrder(expr.AST)

	for _, task := range tasks {
		if err := o.Storage.CreateTask(&storage.Task{
			ID:            task.ID,
			ExprID:        exprID,
			Arg1:          task.Arg1,
			Arg2:          task.Arg2,
			Operation:     task.Operation,
			OperationTime: task.OperationTime,
		}); err != nil {
			log.Printf("Failed to create task: %v", err)
			continue
		}
		o.taskStore[task.ID] = task
		o.taskQueue = append(o.taskQueue, task)
		log.Printf("Created task %s: %.2f %s %.2f",
			task.ID, task.Arg1, task.Operation, task.Arg2)
	}
}

func (o *Orchestrator) RunServer() error {
	lis, err := net.Listen("tcp", ":"+o.Config.GRPCAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterCalculatorServer(grpcServer, &server{o: o})

	go func() {
		log.Printf("запускается gRPC сеовер на порту %s", o.Config.GRPCAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/login", o.loginHandler)
	mux.HandleFunc("/api/v1/register", o.registerHandler)

	protected := http.NewServeMux()
	protected.HandleFunc("/calculate", o.calculateHandler)
	protected.HandleFunc("/expressions", o.expressionsHandler)
	protected.HandleFunc("/expressions/", o.expressionIDHandler)
	protected.HandleFunc("/internal/task", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			o.getTaskHandler(w, r)
		} else if r.Method == http.MethodPost {
			o.postTaskHandler(w, r)
		}
	})

	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", o.authMiddleware(protected)))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"API не найдено"}`, http.StatusNotFound)
	})

	go func() {
		for {
			time.Sleep(2 * time.Second)
			o.mu.Lock()
			if len(o.taskQueue) > 0 {
				log.Printf("Решаем задачи в очереди: %d", len(o.taskQueue))
			}
			o.mu.Unlock()
		}
	}()

	log.Printf("Запускаем HTTP сервер на порту %s", o.Config.HTTPAddr)
	return http.ListenAndServe(":"+o.Config.HTTPAddr, mux)
}
