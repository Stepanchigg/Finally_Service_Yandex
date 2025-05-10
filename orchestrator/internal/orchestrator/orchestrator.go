package orchestrator

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type server struct {
	proto.UnimplementedCalculatorServer
	o *Orchestrator
}

//Config содержит настройки оркестратора из переменных окружения
type Config struct {
	HTTPAddr            string
	GRPCAddr            string
	TimeAddition        int
	TimeSubtraction     int
	TimeMultiplications int
	TimeDivisions       int
}
//ConfigFromEnv создает конфиг из переменных окружения
func ConfigFromEnv() *Config {
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
//Orchestrator управляет выражениями и задачами
type Orchestrator struct {
	Config      *Config
	exprStore   map[string]*Expression
	taskStore   map[string]*Task
	taskQueue   []*Task
	mu          sync.Mutex
	exprCounter int64
	taskCounter int64
}
//NewOrchestrator создает новый экземпляр оркестратора
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		Config:    ConfigFromEnv(),
		exprStore: make(map[string]*Expression),
		taskStore: make(map[string]*Task),
		taskQueue: make([]*Task, 0),
	}
}
//Expression представляет математическое выражение
type Expression struct {
	ID     string   `json:"id"`
	Expr   string   `json:"expression"`
	Status string   `json:"status"`
	Result *float64 `json:"result,omitempty"`
	AST    *ASTNode `json:"-"`
}
//Task представляет вычислительную задачу
type Task struct {
	ID            string   `json:"id"`
	ExprID        string   `json:"-"`
	Arg1          float64  `json:"arg1"`
	Arg2          float64  `json:"arg2"`
	Operation     string   `json:"operation"`
	OperationTime int      `json:"operation_time"`
	Node          *ASTNode `json:"-"`
}
//calculateHandler обрабатывает запрос на вычисление выражения
func (o *Orchestrator) calculateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Неправильный метод"}`, http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Expression string `json:"expression"`
	}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Expression == "" {
		http.Error(w, `{"error":"Невалидное выражение"}`, http.StatusUnprocessableEntity)
		return
	}
	ast, err := ParseAST(req.Expression)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusUnprocessableEntity)
		return
	}
	o.mu.Lock()
	o.exprCounter++
	exprID := fmt.Sprintf("%d", o.exprCounter)
	expr := &Expression{
		ID:     exprID,
		Expr:   req.Expression,
		Status: "обрабатывается",
		AST:    ast,
	}
	o.exprStore[exprID] = expr
	o.scheduleTasks(expr)
	o.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": exprID})
}
//expressionsHandler возвращает список всех выражений
func (o *Orchestrator) expressionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Неправильный метод"}`, http.StatusMethodNotAllowed)
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	exprs := make([]*Expression, 0, len(o.exprStore))
	for _, expr := range o.exprStore {
		if expr.AST != nil && expr.AST.IsLeaf {
			expr.Status = "завершено"
			expr.Result = &expr.AST.Value
		}
		exprs = append(exprs, expr)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"expressions": exprs})
}
//expressionByIDHandler возвращает выражение по ID
func (o *Orchestrator) expressionByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Неправильный метод"}`, http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Path[len("/api/v1/expressions/"):]
	o.mu.Lock()
	expr, ok := o.exprStore[id]
	o.mu.Unlock()
	if !ok {
		http.Error(w, `{"error":"Выражение не найдено"}`, http.StatusNotFound)
		return
	}
	if expr.AST != nil && expr.AST.IsLeaf {
		expr.Status = "завершено"
		expr.Result = &expr.AST.Value
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"expression": expr})
}
//getTaskHandler возвращает следующую задачу из очереди
func (o *Orchestrator) getTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Неправильный метод"}`, http.StatusMethodNotAllowed)
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.taskQueue) == 0 {
		http.Error(w, `{"error":"Нет доступной задачи"}`, http.StatusNotFound)
		return
	}
	task := o.taskQueue[0]
	o.taskQueue = o.taskQueue[1:]
	if expr, exists := o.exprStore[task.ExprID]; exists {
		expr.Status = "В_процессе"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"task": task})
}
//postTaskHandler принимает результаты вычислений
func (o *Orchestrator) postTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Неправильный метод"}`, http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID     string  `json:"id"`
		Result float64 `json:"result"`
	}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.ID == "" {
		http.Error(w, `{"error":"Невалидное тело"}`, http.StatusUnprocessableEntity)
		return
	}
	o.mu.Lock()
	task, ok := o.taskStore[req.ID]
	if !ok {
		o.mu.Unlock()
		http.Error(w, `{"error":"Задача не найдена"}`, http.StatusNotFound)
		return
	}
	task.Node.IsLeaf = true
	task.Node.Value = req.Result
	delete(o.taskStore, req.ID)
	if expr, exists := o.exprStore[task.ExprID]; exists {
		o.scheduleTasks(expr)
		if expr.AST.IsLeaf {
			expr.Status = "завершено"
			expr.Result = &expr.AST.Value
		}
	}
	o.mu.Unlock()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"результат принят"}`))
}
//scheduleTasks планирует задачи для выражения
func (o *Orchestrator) scheduleTasks(expr *Expression) {
	var traverse func(node *ASTNode)
	traverse = func(node *ASTNode) {
		if node == nil || node.IsLeaf {
			return
		}
		traverse(node.Left)
		traverse(node.Right)
		if node.Left != nil && node.Right != nil && node.Left.IsLeaf && node.Right.IsLeaf {
			if !node.TaskScheduled {
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
					Arg1:          node.Left.Value,
					Arg2:          node.Right.Value,
					Operation:     node.Operator,
					OperationTime: opTime,
					Node:          node,
				}
				node.TaskScheduled = true
				o.taskStore[taskID] = task
				o.taskQueue = append(o.taskQueue, task)
			}
		}
	}
	traverse(expr.AST)
}

func (o *Orchestrator) registerHandler(w http.ResponseWriter, r *http.Request){

	//до нижнего надо еще написать добавление в СУБД создание таблицы и тд хз что там еще надо почитай сам будущий я
	//пишу этот через день после верхнего, по сути все функции в crypto.go, осталось шамать с ними + мб через горутины
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Неправильный метод"}`, http.StatusMethodNotAllowed)
		return
	}
	const hmacSampleSecret = "super_secret_signature"// типо должно браться что то вроде хз тип
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"name": "user_name",
		"nbf":  now.Add(time.Minute).Unix(),
		"exp":  now.Add(5 * time.Minute).Unix(),
		"iat":  now.Unix(),
	})

	tokenString, err := token.SignedString([]byte(hmacSampleSecret))
	if err != nil {
		panic(err)
	}

}
func (o *Orchestrator) loginHandler(w http.ResponseWriter, r *http.Request){
	 
}

func (o *Orchestrator) RunServer() error {
	//запросы и мютекс
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/calculate", o.calculateHandler)
	mux.HandleFunc("/api/v1/expressions", o.expressionsHandler)
	mux.HandleFunc("/api/v1/expressions/", o.expressionByIDHandler)
	mux.HandleFunc("/api/v1/register", o.registerHandler)
	//mux.HandleFunc("/api/v1/login", o.loginHandler)
	mux.HandleFunc("/internal/task", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			o.getTaskHandler(w, r)
		} else if r.Method == http.MethodPost {
			o.postTaskHandler(w, r)
		} else {
			http.Error(w, `{"error":"Неправильный метод"}`, http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"Не найдено"}`, http.StatusNotFound)
		
	})
	go func() {
		for {
			time.Sleep(2 * time.Second)
			o.mu.Lock()
			if len(o.taskQueue) > 0 {
				log.Printf("Обрабатываемые задачи в очереди: %d", len(o.taskQueue))
			}
			o.mu.Unlock()
		}
	}()
	return http.ListenAndServe(":"+o.Config.Addr, mux)
}