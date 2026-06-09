# GOrevision

Небольшой HTTP reverse proxy и load balancer на Go.

Идея простая: сервис слушает входящие запросы, выбирает один из доступных backend-ов и проксирует запрос на него. Если backend падает, health-checks помечают его как нерабочий, и новые запросы туда больше не отправляются.

Это не замена настоящему Nginx. Скорее учебная реализация, где руками сделаны основные части: выбор backend-а, проверки доступности, учёт активных соединений и проксирование HTTP-запросов.

## Что умеет

- Принимает HTTP-трафик на заданном порту.
- Распределяет запросы между несколькими backend-серверами.
- Поддерживает три алгоритма балансировки:
  - `round_robin`
  - `least_connections`
  - `ip_hash`
- Проверяет backend-ы в фоне через health-check endpoint.
- Не отправляет трафик на backend, который считается упавшим.
- Работает асинхронно за счёт стандартной HTTP-модели Go: каждый запрос обрабатывается отдельно, без ручного пула потоков.
- Использует keep-alive соединения к backend-ам, чтобы не открывать новый TCP-коннект на каждый запрос.

## Структура

```text
cmd/
  gorevision/       основной запуск балансировщика
  demo-backend/     простой тестовый backend

internal/
  config/           чтение и проверка config.json
  lb/               backend-и, алгоритмы балансировки, health-checks
  proxy/            HTTP handler, который проксирует запросы
```

## Быстрый запуск

Нужен установленный Go.

Сначала скопируй пример конфига:

```powershell
Copy-Item config.example.json config.json
```

Потом в трёх отдельных терминалах запусти тестовые backend-ы:

```powershell
go run ./cmd/demo-backend --port 9001 --name backend-1
```

```powershell
go run ./cmd/demo-backend --port 9002 --name backend-2
```

```powershell
go run ./cmd/demo-backend --port 9003 --name backend-3
```

После этого запусти сам балансировщик:

```powershell
go run ./cmd/gorevision --config config.json
```

Проверка:

```powershell
curl http://localhost:8080/
```

Если стоит `round_robin`, ответы должны идти по очереди от разных backend-ов.

## Конфиг

Пример:

```json
{
  "listen": ":8080",
  "algorithm": "round_robin",
  "health_check": {
    "path": "/health",
    "interval": "2s",
    "timeout": "1s",
    "healthy_threshold": 1,
    "unhealthy_threshold": 2
  },
  "backends": [
    { "url": "http://localhost:9001" },
    { "url": "http://localhost:9002" },
    { "url": "http://localhost:9003" }
  ]
}
```

Поля:

- `listen` - адрес, на котором будет слушать балансировщик.
- `algorithm` - алгоритм выбора backend-а.
- `health_check.path` - путь, который будет проверяться на каждом backend-е.
- `health_check.interval` - как часто делать проверку.
- `health_check.timeout` - сколько ждать ответа от backend-а.
- `healthy_threshold` - сколько успешных проверок нужно, чтобы вернуть backend в работу.
- `unhealthy_threshold` - сколько неудачных проверок нужно, чтобы убрать backend из ротации.
- `backends` - список серверов, между которыми распределяется трафик.

## Алгоритмы

### round_robin

Обычный выбор по кругу.

Если есть три backend-а, запросы идут примерно так:

```text
backend-1 -> backend-2 -> backend-3 -> backend-1 -> ...
```

Если один backend помечен как нерабочий, он пропускается.

### least_connections

Выбирает backend с минимальным количеством активных запросов.

Это полезно, если часть запросов обрабатывается дольше остальных. Например, один backend уже занят тяжёлыми запросами, а второй свободен. Тогда новый запрос уйдёт на второй.

### ip_hash

Выбирает backend по IP клиента.

Один и тот же клиент обычно попадает на один и тот же backend, пока набор живых backend-ов не меняется. Это может пригодиться, если backend хранит часть состояния локально.

IP берётся из:

- `X-Forwarded-For`
- `X-Real-IP`
- `RemoteAddr`

## Health-checks

Балансировщик периодически делает `GET` на health-check путь каждого backend-а.

Backend считается живым, если ответил статусом `2xx` или `3xx`.

Если backend не отвечает, отвечает слишком долго или возвращает ошибочный статус, он получает неудачную проверку. После нужного количества неудач он убирается из балансировки.

Пример: при таком конфиге backend отключится после двух подряд неудачных проверок:

```json
"unhealthy_threshold": 2
```

Когда backend снова начнёт отвечать нормально, он вернётся в работу после нужного количества успешных проверок.

## Проверка падения backend-а

Запусти три backend-а и балансировщик, потом останови один backend, например на порту `9002`.

Через пару секунд в логах балансировщика будет примерно такое:

```text
backend http://localhost:9002 marked unhealthy
```

После этого запросы через `localhost:8080` должны идти только на оставшиеся backend-ы.

## Сборка

```powershell
go build -o gorevision.exe ./cmd/gorevision
```

Тестовый backend:

```powershell
go build -o demo-backend.exe ./cmd/demo-backend
```

Запуск собранного файла:

```powershell
.\gorevision.exe --config config.json
```

## Тесты

```powershell
go test ./...
```

Сейчас тестами покрыты основные алгоритмы выбора backend-а:

- `round_robin` не выбирает нерабочие backend-ы;
- `least_connections` выбирает backend с меньшим числом активных соединений;
- `ip_hash` стабильно выбирает один backend для одного клиента.

## Ограничения

Сервис работает только как HTTP reverse proxy. HTTPS termination, HTTP/2-тюнинг, rate limits, retries, access logs и метрики здесь не добавлены.

Это можно дописать отдельно, но базовая логика балансировщика уже вынесена в отдельные пакеты, так что расширять проект несложно.
