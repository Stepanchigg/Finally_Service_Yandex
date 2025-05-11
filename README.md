# Finally_Service_Yandex
Итоговая задача модуля 5 Яндекс лицея.

Инструкция по запуску:

1)Убедитесь, что у вас установлен Go (версия 1.16 или выше).

2)Убедитесь, что у вас установлен SQLite3(можно командой для windows через powershell(winget install --id=SQLite.SQLite  -e)).

3)Включите CGO (если выключен) командой export CGO_ENABLED=1.

4)Скачайте gcc compiler.(гайд есть на сайте. убедитесь что используете правильную 64-bit модель(https://www.codewithharry.com/blogpost/how-to-install-gnu-gcc-compiler-on-windows))

5)Скопируйте репозиторий(через git bash):

```bash
git clone https://github.com/Stepanchigg/Finally_Service_Yandex
```

```bash
cd Finally_Service_Yandex
```

Запускаем orchestator:

```bash
export ORCHESTRATOR_URL=localhost:50051
export TIME_ADDITION_MS=200
export TIME_SUBTRACTION_MS=200
export TIME_MULTIPLICATIONS_MS=300
export TIME_DIVISIONS_MS=400

go run cmd/orchestrator/orchestrator_start.go
```

Вы получите ответ:
2025/05/12 00:30:25 Запускаем Orchestrator на порту 8080
2025/05/12 00:30:25 Запускаем HTTP сервер на порту 8080
2025/05/12 00:30:25 Запускаем gRPC сервер на порту 50051

В новом окне git bash:

Опять переходим в репозиторию с проектом:

```bash
cd Finally_Service_Yandex
```

Затем запускаем agent:

```bash
export COMPUTING_POWER=4
export ORCHESTRATOR_URL=localhost:50051

 go run cmd/agent/agent_start.go
```

Вы получите ответ:
2025/05/12 01:17:05 Запусаем Agent...
2025/05/12 01:17:05 Запускается worker 0
2025/05/12 01:17:05 Запускается worker 1
2025/05/12 01:17:05 Запускается worker 2
2025/05/12 01:17:05 Запускается worker 3

2025/05/12 01:16:09 Worker 0: ошибка в получении задачи: rpc error: code = Unknown desc = not found
2025/05/12 01:16:09 Worker 1: ошибка в получении задачи: rpc error: code = Unknown desc = not found
2025/05/12 01:16:09 Worker 3: ошибка в получении задачи: rpc error: code = Unknown desc = not found
2025/05/12 01:16:11 Worker 2: ошибка в получении задачи: rpc error: code = Unknown desc = not found
(это потому что нет активных задач)

Регестрируем нового пользователя:

```bash
curl -X POST http://localhost:8080/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{"login":"roflan","password":"123567"}'
```

Входим как пользователь:

```bash
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"login":"roflan","password":"123567"}'
```

далее при запуске надо будет использовать свой токен 

```bash
curl --location 'http://localhost:8080/api/v1/calculate' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer (здесь JWT коин, Bearer не трогаем)' \
--data '{"expression": "2+2*2"}'
```

Примеры использования:

Успешный запрос:

```bash
curl --location 'http://localhost:8080/api/v1/calculate' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer (здесь JWT коин, Bearer не трогаем)' \
--data '{"expression": "2+2*2"}'
```

Ответ:

```bash
{
  "id": "..."
}
```

После можно посмотреть этап выполнения данного запроса:

```bash
curl --location 'http://localhost:8080/api/v1/expressions' \
--header 'Authorization: Bearer (здесь JWT коин, Bearer не трогаем)'
```

Вывод:

```bash
{"expressions":[{"id":"1","expression":"2*2+2,"status":"pending"}]}
```

Если вычисления выполнены то:

```bash
{"expression":{"id":"1","expression":"2*2+2","result":6,"status":"completed"}}
```

Ошибки при запросах:

Ошибка при создании пользователя который уже существует:

```bash
{"error":"Пользователь уже существует"}
```

Ошибка 404(отсутствие выражения ):

```bash
{"error":"API Not Found"}
```

Ошибка 422 (невалидное выражение ):

```bash
curl --location 'http://localhost:8080/api/v1/calculate' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer (здесь JWT коин, Bearer не трогаем)' \
--data '
{
  "expression": "2+a"
}'

```
Ответ:

```bash
{
  {"error":"неожиданное число на месте 2"}
}
```

Ошибка 500 (внутренняя ошибка сервера ):

```bash
curl --location 'http://localhost:8080/api/v1/calculate' \
--header 'Content-Type: application/json' \
--data '
{
  "expression": "2/0"
}'
```
Ответ(у  меня высвечивается изначально id созданной задачи,а в окне git bash где был запущен agent можно увидеть что выводится деление на 0 ):

```bash
{
  Worker: error computing task : division by zero
}
```

Тесты запускаются git bash:

1)Сначала опять переходим в папку с модулем.

```bash
cd Finally_Service_Yandex
```

2)Затем запускаем тестирование:

Для агента

```bash
go test ./internal/agent/agent_test.go
```

Интеграционный

```bash
go test ./cmd/inter_test.go
```

Для Storage

```bash
go test ./internal/storage/storage_test.go
```

3)При успешном прохождение теста должен вывестись ответ:

```bash
ok  	calc_service/internal/evaluator	0.001s
```

4)При ошибке в тестах будет указано где она совершена.
Кстати. Ошибка в тесте агента связанная с не указанным ErrDivivsionByZero появляется так как в функции тестирования я ее не оглашаю,
она создает конфликты (в VSC как минимум) так как уже в самом агенте
