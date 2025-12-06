# Пример Echo-сервера на базе Hioload-WS

Этот пакет содержит **Echo WebSocket Server** на Go, демонстрирующий возможности библиотеки `hioload-ws` для сверхвысоконагруженных WebSocket-приложений. Сервер принимает бинарные фреймы от клиентов и возвращает их обратно без аллокаций и копирований, используя zero-copy, NUMA-aware буферные пулы и batch I/O реактор.

## Содержание

1. [Обзор](#обзор)  
2. [Основные возможности](#основные-возможности)  
3. [Требования](#требования)  
4. [Установка](#установка)  
5. [Параметры конфигурации](#параметры-конфигурации)  
6. [Архитектура и дизайн](#архитектура-и-дизайн)  
   6.1 [Паттерн Фасада](#паттерн-фасада)  
   6.2 [Цепочка Middleware](#цепочка-middleware)  
   6.3 [Zero-Copy Буферы](#zero-copy-буферы)  
   6.4 [Batch I/O Reactor](#batch-io-reactor)  
   6.5 [NUMA-Aware Affinity](#numa-aware-affinity)  
   6.6 [Lock-Free Структуры](#lock-free-структуры)  
7. [Разбор кода](#разбор-кода)  
   7.1 [main.go](#maingo)  
   7.2 [go.mod и go.sum](#gomod--gosum)  
8. [Запуск примера](#запуск-примера)  
9. [Тонкая настройка производительности](#тонкая-настройка-производительности)  
10. [Тестирование и проверка](#тестирование-и-проверка)  
11. [Расширение примера](#расширение-примера)  
12. [Отладка и метрики](#отладка-и-метрики)  
13. [Грейсфул-шатдаун](#грейсфул-шатдаун)  
14. [Лицензия](#лицензия)  
15. [Контакты и вклад в проект](#контакты-и-вклад-в-проект)  

---

## Обзор

Пример Echo-сервера показывает:

- Настройку и запуск `hioload-ws` **Server facade**.  
- Использование **zero-copy** NUMA-aware пула буферов.  
- Конфигурацию **batch-mode** реактора (PollerAdapter).  
- Реализацию простого **echo handler**, возвращающего те же данные клиенту.  
- Интеграцию **middleware** для логирования, восстановления после паник и подсчёта сообщений.  
- Привязку потоков и рабочих к NUMA-узлам и ядрам CPU для минимальной латентности.  
- Грейсфул-шатдаун при получении SIGINT/SIGTERM.

---

## Основные возможности

- **Zero-Copy**: данные передаются без дополнительных копирований и аллокаций.  
- **Batch I/O Reactor**: обработка пачек событий за один проход.  
- **NUMA-Aware Affinity**: пининг потоков к узлам NUMA.  
- **Lock-Free Структуры**: ring buffer и lock-free очереди для минимальной конкуренции.  
- **Middleware Chain**: легко расширяемые цепочки middleware.  
- **Грейсфул-шатдаун**: корректное завершение работы и очистка ресурсов.  
- **Кросс-платформенность**: Linux (epoll, io_uring) и Windows (IOCP).

---

## Требования

- Go ≥ 1.23  
- Linux ядро ≥ 6.20 или Windows Server ≥ 2016 (amd64)  
- Доступ к модулю `github.com/momentics/hioload-ws`  
- Знания Go, goroutine и сетевых API

---

## Установка

Клонирование и подготовка:

```

git clone https://github.com/momentics/hioload-ws.git
cd hioload-ws/examples/lowlevel/echo
go mod tidy

```

---

## Параметры конфигурации

Флаги командной строки:

| Флаг          | По умолчанию | Описание                                                      |
|---------------|--------------|---------------------------------------------------------------|
| `-addr`       | `:9001`      | TCP-адрес для WebSocket-сервера                               |
| `-batch`      | `32`         | Размер пачки событий в реакторе                               |
| `-ring`       | `1024`       | Емкость кольцевого буфера реактора                            |
| `-workers`    | `0`          | Количество воркеров (0 = runtime.NumCPU())                    |
| `-numa`       | `-1`         | NUMA-узел для пули буферов и пининга потоков (-1 = авто)      |

Запуск с настройками:

```

go run main.go -addr=":9001" -batch=64 -ring=2048 -workers=8 -numa=0

```

---

## Архитектура и дизайн

### Паттерн Фасада

`server.NewServer(cfg, opts...)` создаёт Server facade:

1. **ControlAdapter** – управление конфигом, метрики, debug-пробы.  
2. **BufferPoolManager** – NUMA-aware zero-copy буферы.  
3. **WebSocketListener** – приём и handshake TCP/WebSocket.  
4. **PollerAdapter** – reactor loop с batch-обработкой.  
5. **ExecutorAdapter** – пул воркеров для фоновых задач.

API facade:

```

srv, err := server.NewServer(cfg, server.WithMiddleware(...))

```

### Цепочка Middleware

Middleware-цепочка оборачивает `api.Handler`:

- `LoggingMiddleware` – вывод логов о типе данных и ошибках.  
- `RecoveryMiddleware` – восстановление после паник внутри handler.  
- `HandlerMetricsMiddleware` – инкремент счётчика `handler.processed`.

Пример подключения:

```

srv.UseMiddleware(
adapters.LoggingMiddleware,
adapters.RecoveryMiddleware,
adapters.HandlerMetricsMiddleware(srv.GetControl()),
)

```

### Zero-Copy Буферы

Пулы буферов (`slabPool`) используют:

- **HugePages** на Linux (mmap MAP_HUGETLB)  
- **VirtualAllocExNuma** на Windows (MEM_LARGE_PAGES)  
- Фиксированные size-class (2K–1M) для эффективного кеширования  
- Возврат буфера через `Buffer.Release()` без копирования

### Batch I/O Reactor

`PollerAdapter` и `EventLoop`:

- Чтение до `BatchSize` событий за цикл  
- Обработка через срез обработчиков без mutex в hot path  
- Адаптивный backoff при отсутствии событий

### NUMA-Aware Affinity

В `Server.Run` пинится основной поток:

```

aff := adapters.NewAffinityAdapter()
aff.Pin(-1, cfg.NUMANode)
defer aff.Unpin()

```

Воркеры executor также пинятся по узлам NUMA.

### Lock-Free Структуры

- **RingBuffer** – concurrent FIFO с atomic head/tail  
- **LockFreeQueue** – очередь SPSC для задач  
- **SlabPool** – lock-free стек свободных буферов

---

## Разбор кода

### main.go

```

package main

import (
"flag"
"fmt"
"os"
"os/signal"
"sync/atomic"
"syscall"
"time"

    "github.com/momentics/hioload-ws/adapters"
    "github.com/momentics/hioload-ws/api"
    "github.com/momentics/hioload-ws/core/protocol"
    "github.com/momentics/hioload-ws/lowlevel/server"
    )

func main() {
// Парсинг флагов
addr := flag.String("addr", ":9001", "WebSocket listen address")
batch := flag.Int("batch", 32, "Reactor batch size")
ring := flag.Int("ring", 1024, "Reactor ring capacity")
workers := flag.Int("workers", 0, "Executor worker count (0 = NumCPU)")
numa := flag.Int("numa", -1, "Preferred NUMA node (-1 = auto)")
flag.Parse()

    // Конфигурация сервера
    cfg := server.DefaultConfig()
    cfg.ListenAddr = *addr
    cfg.BatchSize = *batch
    cfg.ReactorRing = *ring
    if *workers > 0 {
        cfg.ExecutorWorkers = *workers
    }
    cfg.NUMANode = *numa
    
    // Инициализация сервера с middleware
    srv, err := server.NewServer(
        cfg,
        server.WithMiddleware(
            adapters.LoggingMiddleware,
            adapters.RecoveryMiddleware,
        ),
    )
    if err != nil {
        fmt.Fprintf(os.Stderr, "NewServer error: %v\n", err)
        os.Exit(1)
    }
    
    // Метрики
    srv.UseMiddleware(adapters.HandlerMetricsMiddleware(srv.GetControl()))
    
    var activeConns int64
    var totalMsgs int64
    ctrl := srv.GetControl()
    ctrl.RegisterDebugProbe("active_connections", func() any { return atomic.LoadInt64(&activeConns) })
    ctrl.RegisterDebugProbe("messages_processed", func() any { return atomic.LoadInt64(&totalMsgs) })
    
    // Периодический вывод статистики
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        defer ticker.Stop()
        for range ticker.C {
            stats := ctrl.Stats()
            fmt.Printf("[%s] Active: %v, Msgs: %v\n",
                time.Now().Format(time.Stamp), stats["active_connections"], stats["messages_processed"])
        }
    }()
    
    // Echo handler
    echoHandler := adapters.HandlerFunc(func(data any) error {
        buf := data.(api.Buffer)
        atomic.AddInt64(&totalMsgs, 1)
        conn := api.FromContext(api.ContextFromData(data)).(*protocol.WSConnection)
        frame := &protocol.WSFrame{IsFinal: true, Opcode: protocol.OpcodeBinary,
            PayloadLen: int64(len(buf.Bytes())), Payload: buf.Bytes()}
        buf.Release()
        return conn.SendFrame(frame)
    })
    
    // Middleware для учета соединений
    track := func(next api.Handler) api.Handler {
        return adapters.HandlerFunc(func(data any) error {
            switch evt := data.(type) {
            case api.OpenEvent:
                atomic.AddInt64(&activeConns, 1)
                defer next.Handle(evt)
            case api.CloseEvent:
                atomic.AddInt64(&activeConns, -1)
                defer next.Handle(evt)
            default:
                return next.Handle(data)
            }
            return nil
        })
    }
    srv.UseMiddleware(track)
    
    // Запуск сервера
    go func() {
        if err := srv.Run(echoHandler); err != nil {
            fmt.Fprintf(os.Stderr, "Run error: %v\n", err)
            os.Exit(1)
        }
    }()
    
    // Грейсфул-шатдаун
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    fmt.Println("Shutting down echo server...")
    srv.Shutdown()
    fmt.Println("Server stopped.")
    }

```

### go.mod и go.sum

```

module github.com/momentics/hioload-ws/examples/lowlevel/echo

go 1.23

require github.com/momentics/hioload-ws v0.0.0

```

---

## Запуск примера

```

cd examples/lowlevel/echo
go run main.go

```

Подключение через `wscat`:

```

wscat -c ws://localhost:9001
> Привет
< Привет

```

---

## Тонкая настройка производительности

- Подбирайте оптимальный `batch` и `ring` для вашего сценария.  
- Привязывайте воркеры к NUMA-узлам для уменьшения латентности памяти.  
- Количество воркеров должно совпадать с количеством ядер CPU или их групп.  
- Мониторьте загрузку CPU, p99-показатели и GC-паузы.

---

## Тестирование и проверка

- Модульные тесты в `api/`, `internal/` проверяют корректность и соглашения.  
- Интеграционные тесты эмулируют echo-трафик.  
- Запуск: `go test ./...`  
- Бенчмарки: `go test -bench .`

---

## Отладка и метрики

- Регистрируйте произвольные debug-пробы:  
```

srv.GetControl().RegisterDebugProbe("my_probe", func() any { return myValue })

```
- Экспортируйте метрики через HTTP или Prometheus.  
- Используйте `net/http/pprof` и трассировку Go.

---

## Grasefull Shutdown

Обработка SIGINT/SIGTERM:

1. Закрытие listener → остановка приема новых соединений.  
2. Закрытие `PollerAdapter` → остановка реактора.  
3. Остановка воркеров executor.  
4. Закрытие буферов и соединений.

---

## Лицензия

Пример распространяется под **Apache License 2.0**. Подробнее — см. [LICENSE](../../LICENSE).

---

## Контакты и вклад в проект

- **Автор**: momentics <momentics@gmail.com>  
- **Репозиторий**: https://github.com/momentics/hioload-ws  
- Вопросы, багрепорты и pull request’ы приветствуются!
