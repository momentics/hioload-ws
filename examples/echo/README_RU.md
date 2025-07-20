# Пример «Echo WebSocket Server» для hioload-ws

## Оглавление

1. Введение  
2. Цели и назначение примера  
3. Описание архитектуры  
   3.1. Facade (унифицированная точка входа)  
   3.2. Транспорт (TCP/DPDK)  
   3.3. BufferPool (NUMA-aware пулы буферов)  
   3.4. Poller и EventLoop (батчинг, zero-copy)  
   3.5. Middleware (логирование, recovery, метрики)  
   3.6. Протокол WebSocket (RFC-6455)  
4. Структура проекта  
   4.1. Файлы и каталоги  
   4.2. go.mod и зависимости  
5. Подробное описание файлов  
   5.1. main.go  
   5.2. go.mod  
6. Инструкция по запуску  
   6.1. Предварительные требования  
   6.2. Сборка и запуск  
   6.3. Примеры работы и тесты  
7. Обзор возможностей hioload-ws на примере Echo  
   7.1. Cross-platform поддержка (Linux/Windows)  
   7.2. Опциональный DPDK  
   7.3. NUMA-aware буферизация  
   7.4. Zero-copy WebSocket  
   7.5. Batch-обработка и Poll-mode IO  
   7.6. Пинning CPU/NUMA  
   7.7. Hot-reload конфигурации и debug-пробы  
   7.8. Graceful shutdown  
8. Рекомендации по расширению  
9. Заключение  

---

## 1. Введение

Этот пример демонстрирует, как на базе библиотеки **hioload-ws** реализовать высокопроизводительный WebSocket Echo-сервер. Все входящие сообщения от клиентов возвращаются обратно («эхо»), при этом используется:

- NUMA-aware пулы буферов для zero-copy передачи;
- Batched poll-mode event loop для минимизации системных вызовов;
- Cross-platform транспорт: Linux (epoll, sendmsg), Windows (IOCP) и опционально DPDK;
- CPU/NUMA affinity pinning для локальности памяти и CPU;
- Расширяемая цепочка middleware: логирование, восстановление после паник, сбор метрик;
- Runtime-конфигурирование и debug-пробы;
- Graceful shutdown по SIGINT/SIGTERM.

---

## 2. Цели и назначение примера

1. Показать базовый паттерн использования `facade.New` и `facade.DefaultConfig` для создания сервера.  
2. Продемонстрировать рабочий цикл WebSocket: HTTP upgrade → NewWSConnection → Start → Recv/Send.  
3. Иллюстрировать управление памятью через `BufferPool` с учётом NUMA.  
4. Использовать middleware-цепочку `LoggingMiddleware`, `RecoveryMiddleware`, `MetricsMiddleware`.  
5. Обработать сигналы ОС и выполнить корректную остановку сервера.  
6. Показать динамическую регистрацию debug-проб через `RegisterDebugProbe`.  

---

## 3. Описание архитектуры

### 3.1. Facade

В `main.go` создаётся экземпляр `HioloadWS` через:

```

cfg := facade.DefaultConfig()
cfg.ListenAddr = *addr
cfg.UseDPDK = *useDPDK
cfg.SessionShards = *shardCount
cfg.NumWorkers = *numWorkers

hioload, err := facade.New(cfg)

```

- **DefaultConfig** задаёт sane-значения: 4 воркера, ringCapacity=1024, batchSize=16, CPUAffinity=true.
- **New** инициализирует ControlAdapter, AffinityAdapter, BufferPoolManager, Transport (DPDK или native), SessionManager, Executor, PollerAdapter.

### 3.2. Транспорт

- **config.UseDPDK**: при сборке с tag `dpdk` и наличии биндингов, будет инициализирован DPDK-транспорт `NewDPDKTransport`.
- Иначе применяется **NewTransport** → platform-specific implementation:
  - Linux: epoll + SendmsgBuffers/RecvmsgBuffers.
  - Windows: IOCP + WSASend/WSARecv.

### 3.3. BufferPool

- `pool.NewBufferPoolManager().GetPool(numaNode)` создаёт пул буферов для указанного NUMA-узла.
- Буферы выделяются через `BufferPool.Get(size, numaPreferred)` и возвращаются вызовом `Release()`.

### 3.4. Poller и EventLoop

- **PollerAdapter** оборачивает `concurrency.EventLoop(batchSize, ringCapacity)`.
- При вызове `RegisterHandler` регистрируется handlerBridge, конвертирующий `[]byte` или `api.Buffer` в вызов `api.Handler.Handle`.
- Batch обработки снижает число системных вызовов и накладных расходов.

### 3.5. Middleware

- **LoggingMiddleware**: логирует тип данных и ошибки.  
- **RecoveryMiddleware**: восстанавливает горутину при панике handler’а.  
- **MetricsMiddleware**: инкрементирует счётчик обработанных сообщений через `Control.SetConfig`.  

### 3.6. Протокол WebSocket

- HTTP endpoint `/ws` → `protocol.UpgradeToWebSocket(r)` → `WSConnection`.
- `WSConnection.Recv()` возвращает `[]api.Buffer` zero-copy.
- `WSConnection.Send(frame)` отправляет через `transport.Send`.

---

## 4. Структура проекта

```

examples/echo/
│
├─ go.mod
└─ main.go

```

### 4.1. Файлы и каталоги

- **go.mod**: описание модуля и зависимостей (`require github.com/momentics/hioload-ws v0.0.0`)
- **main.go**: реализация Echo-сервера.

### 4.2. go.mod и зависимости

```

module github.com/momentics/hioload-ws/examples/echo
go 1.23
require github.com/momentics/hioload-ws v0.0.0

```

---

## 5. Подробное описание файлов

### 5.1. main.go

```

package main

import (
...
"github.com/momentics/hioload-ws/facade"
"github.com/momentics/hioload-ws/adapters"
"github.com/momentics/hioload-ws/protocol"
)

func main() {
// Парсинг флагов
addr := flag.String("addr", ":8080", "listen address")
useDPDK := flag.Bool("dpdk", false, "enable DPDK transport")
shards := flag.Int("shards", 16, "session shards")
workers := flag.Int("workers", 4, "executor workers")
flag.Parse()

    // Конфигурация
    cfg := facade.DefaultConfig()
    cfg.ListenAddr = *addr
    cfg.UseDPDK = *useDPDK
    cfg.SessionShards = *shards
    cfg.NumWorkers = *workers
    
    // Создание фасада
    hioload, err := facade.New(cfg)
    if err != nil {
        log.Fatalf("New hioload-ws: %v", err)
    }
    
    // Регистрация debug-пробы
    hioload.GetControl().RegisterDebugProbe("active_sessions", func() any {
        return fmt.Sprintf("%d", hioload.GetSessionCount())
    })
    
    // Запуск
    if err := hioload.Start(); err != nil {
        log.Fatalf("Start failed: %v", err)
    }
    log.Printf("Server listening on %s (DPDK=%v)", cfg.ListenAddr, cfg.UseDPDK)
    
    // Middleware и handler
    base := &EchoHandler{hioload: hioload}
    mw := adapters.NewMiddlewareHandler(base).
          Use(adapters.LoggingMiddleware).
          Use(adapters.RecoveryMiddleware).
          Use(adapters.MetricsMiddleware(hioload.GetControl()))
    
    if err := hioload.RegisterHandler(mw); err != nil {
        log.Fatalf("RegisterHandler failed: %v", err)
    }
    
    // HTTP upgrade
    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        headers, err := protocol.UpgradeToWebSocket(r)
        ...
        conn := hioload.CreateWebSocketConnection()
        conn.SetHandler(mw)
        conn.Start()
        log.Printf("Client connected: %s", r.RemoteAddr)
    })
    
    // Graceful shutdown
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    log.Println("Shutting down...")
    hioload.Stop()
    }

```

#### EchoHandler

```

type EchoHandler struct {
hioload *facade.HioloadWS
}

func (h *EchoHandler) Handle(data any) error {
buf, ok := data.([]byte)
if !ok { return nil }
out := h.hioload.GetBuffer(len(buf))
copy(out.Bytes(), buf)
h.hioload.GetTransport().Send([][]byte{out.Bytes()})
out.Release()
return nil
}

```

### 5.2. go.mod

см. раздел 4.2.

---

## 6. Инструкция по запуску

### 6.1. Предварительные требования

- Go ≥1.23  
- Linux или Windows, архитектура amd64  
- Для DPDK: установленные библиотеки DPDK, сборка с `-tags=dpdk`  

### 6.2. Сборка и запуск

```

git clone https://github.com/momentics/hioload-ws.git
cd hioload-ws/examples/echo
go mod tidy
go run main.go -addr=":8080" -workers=8 -shards=16

# или с DPDK

go run -tags=dpdk main.go -dpdk=true

```

### 6.3. Примеры работы и тесты

Подключитесь через `wscat`:

```

wscat -c ws://localhost:8080/ws
> Hello
< Hello

```

---

## 7. Обзор возможностей hioload-ws на примере Echo

### 7.1. Cross-platform поддержка

Пример одинаково компилируется и запускается на Linux и Windows.

### 7.2. Опциональный DPDK

Флаг `-dpdk` и сборка с `-tags=dpdk` активирует DPDK-транспорт.

### 7.3. NUMA-aware буферизация

Пулы буферов сегментированы по NUMA узлам; по-умолчанию используется узел `NUMANode = -1`.

### 7.4. Zero-copy WebSocket

`protocol.UpgradeToWebSocket` и `WSConnection.Recv()` работают через `api.Buffer`, обеспечивая отсутствие ненужных аллокаций.

### 7.5. Batch-обработка и Poll-mode IO

Входящие события обрабатываются батчами в `EventLoop`, уменьшая overhead.

### 7.6. CPU/NUMA pinning

`DefaultConfig().CPUAffinity = true` автоматически пиннит event loop на NUMA и CPU.

### 7.7. Hot-reload конфигурации и debug-пробы

Через `ControlAdapter` можно регистрировать debug-пробы, например `active_sessions`.

### 7.8. Graceful shutdown

SIGINT/SIGTERM инициируют корректное завершение: остановка poller, transport, executor, отлип affinity.

---
