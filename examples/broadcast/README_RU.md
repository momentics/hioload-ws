# Пример «Broadcast/Чат-сервер» для hioload-ws

---

## 1. Введение

Пример **Broadcast/Чат-сервер** показывает, как с помощью **hioload-ws** создать сервер, принимающий сообщения от любого клиента и моментально рассылающий их всем подключённым пользователям.  

Особенности:
- Zero-copy NUMA-aware буферизация
- Батч-обработка и Poll-mode Reactor
- Управление списком сессий (SessionManager)
- Опциональный DPDK-транспорт
- CPU/NUMA affinity
- Middleware: логирование, восстановление от паник, метрики
- Runtime debug-проба «active_clients»
- Graceful shutdown по SIGINT/SIGTERM

---

## 2. Задачи и сценарии использования

- **Чат в реальном времени**: быстрый обмен сообщениями между множеством клиентов.  
- **Broadcast уведомления**: рассылка системных сообщений всем подписчикам.  
- **Load-test**: проверка производительности при большом количестве подключений.  
- **Architectural demo**: показывает управление сессиями и масштабируемость.  

---

## 3. Архитектура решения

### 3.1. Facade и конфигурация

```

cfg := facade.DefaultConfig()
cfg.ListenAddr = *addr
cfg.UseDPDK = *useDPDK
cfg.SessionShards = *shards
cfg.NumWorkers = *workers

hioload, err := facade.New(cfg)

```

- `SessionShards` определяет количество шард для распределения сессий.
- `NumWorkers` — число воркеров Executor’а.
- `UseDPDK` позволяет включить DPDK-транспорт.

### 3.2. ChatHandler и управление сессиями

```

type ChatHandler struct {
mu       sync.RWMutex
sessions map[string]*protocol.WSConnection
hioload  *facade.HioloadWS
}

```

- `AddClient(id, conn)` добавляет новую сессию.  
- `RemoveClient(id)` удаляет сессию после закрытия.  
- `Handle(data)` рассылает каждый буфер `data []byte` всем активным соединениям:  
  1. Копирует данные в новый zero-copy буфер `GetBuffer`.  
  2. Отсылает через `conn.Send`.

### 3.3. Протокол WebSocket и WSConnection

1. HTTP-handler `/ws` → `protocol.UpgradeToWebSocket`.  
2. Создание `WSConnection` через `hioload.CreateWebSocketConnection()`.  
3. `conn.SetHandler(mw)` и `conn.Start()`.  
4. В отдельной горутине ожидаем `<-conn.Done()` и вызываем `RemoveClient`.

### 3.4. BufferPool и zero-copy

- Буферы выделяются через `hioload.GetBuffer(len, numaNode)`.  
- Zero-copy: при отправке копирование происходит только при первом создании Slicing.  
- По завершении работы с буфером вызывается `Release()`.

### 3.5. Poller и EventLoop

- `hioload.RegisterHandler(mw)` регистрирует middleware-адаптированный handler.  
- `PollerAdapter` запускает `EventLoop` автоматически при первом `Poll`.

### 3.6. Middleware

```

mw := adapters.NewMiddlewareHandler(chatHandler).
Use(adapters.LoggingMiddleware).
Use(adapters.RecoveryMiddleware).
Use(adapters.MetricsMiddleware(hioload.GetControl()))

```

- **LoggingMiddleware** логирует каждое сообщение.  
- **RecoveryMiddleware** защищает от паник.  
- **MetricsMiddleware** инкрементирует счётчик обработанных сообщений.

### 3.7. Debug-пробы

```

hioload.GetControl().RegisterDebugProbe("active_clients", func() any {
return fmt.Sprintf("%d", len(chatHandler.sessions))
})

```

- В любой момент можно получить значение `active_clients` через `Control.Stats()`.

### 3.8. Graceful shutdown

- Сигналы SIGINT, SIGTERM ловятся и инициируют `hioload.Stop()`.  
- Останавливаются: poller, transport, executor, снимается affinity.

---

## 4. Структура проекта

```

examples/broadcast/
│
├─ go.mod
└─ main.go

```

### 4.1. Каталог examples/broadcast

- **main.go** — исходник чат-сервера.  
- **go.mod** — модуль и зависимость `github.com/momentics/hioload-ws`.

### 4.2. go.mod

```

module github.com/momentics/hioload-ws/examples/broadcast
go 1.23
require github.com/momentics/hioload-ws v0.0.0

```

---

## 5. Описание ключевых файлов

### 5.1. main.go

```

package main

import (
"flag"; "fmt"; "log"; "net/http"; "os"; "os/signal"; "sync"; "syscall"
"github.com/momentics/hioload-ws/facade"
"github.com/momentics/hioload-ws/adapters"
"github.com/momentics/hioload-ws/protocol"
)

func main() {
addr := flag.String("addr", ":8080", "HTTP listen address")
useDPDK := flag.Bool("dpdk", false, "enable DPDK transport")
shards := flag.Int("shards", 16, "session shards")
workers := flag.Int("workers", 4, "executor workers")
flag.Parse()

    cfg := facade.DefaultConfig()
    cfg.ListenAddr = *addr
    cfg.UseDPDK = *useDPDK
    cfg.SessionShards = *shards
    cfg.NumWorkers = *workers
    
    hioload, err := facade.New(cfg)
    if err != nil {
        log.Fatalf("Failed to create HioloadWS: %v", err)
    }
    
    chatHandler := NewChatHandler(hioload)
    hioload.GetControl().RegisterDebugProbe("active_clients", func() any {
        return fmt.Sprintf("%d", len(chatHandler.sessions))
    })
    
    if err := hioload.Start(); err != nil {
        log.Fatalf("Start error: %v", err)
    }
    log.Printf("Broadcast server listening on %s (DPDK=%v)", cfg.ListenAddr, cfg.UseDPDK)
    
    mw := adapters.NewMiddlewareHandler(chatHandler).
        Use(adapters.LoggingMiddleware).
        Use(adapters.RecoveryMiddleware).
        Use(adapters.MetricsMiddleware(hioload.GetControl()))
    
    hioload.RegisterHandler(mw)
    
    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        headers, err := protocol.UpgradeToWebSocket(r)
        if err != nil {
            http.Error(w, "Upgrade failed", http.StatusBadRequest)
            return
        }
        for k, v := range headers { w.Header()[k] = v }
        w.WriteHeader(http.StatusSwitchingProtocols)
    
        conn := hioload.CreateWebSocketConnection()
        conn.SetHandler(mw)
        chatHandler.AddClient(r.RemoteAddr, conn)
        go func(id string, c *protocol.WSConnection) {
            c.Start()
            <-c.Done()
            chatHandler.RemoveClient(id)
        }(r.RemoteAddr, conn)
    })
    
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    log.Println("Shutting down...")
    hioload.Stop()
    }

```

### 5.2. go.mod

см. раздел 4.2.

---

## 6. Инструкция по запуску и тестированию

### 6.1. Требования

- Go ≥1.23, amd64  
- Linux или Windows  
- Опционально DPDK  

### 6.2. Запуск сервера

```

cd examples/broadcast
go mod tidy
go run main.go -addr=":8080" -workers=8 -shards=16

# или с DPDK

go run -tags=dpdk main.go -dpdk=true

```

### 6.3. Подключение клиентов

Используйте `wscat` или браузер:

```

wscat -c ws://localhost:8080/ws
> Hello everyone!
< Hello everyone!

```

Запустите несколько клиентов — сообщения будут транслироваться всем.

---

## 7. Обзор возможностей hioload-ws через чат-сервер

1. **SessionManager** — распределённое по шартам хранение сессий, lock-free хранилище с FNV-хешированием.  
2. **Message broadcast** — efficient batch send через zero-copy буферы и `Transport.Send`.  
3. **Affinity** — закрепление потоков на CPU/NUMA.  
4. **Metrics & debug** — отслеживание `active_clients` в режиме реального времени.  
5. **Middleware** — легко добавлять новые слои (например, авторизацию).  
6. **Cross-platform** — одинаковое поведение на Linux/Windows, опциональный DPDK.  

---

