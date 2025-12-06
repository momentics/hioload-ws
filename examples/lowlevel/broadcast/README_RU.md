# Пример широковещательного WebSocket-сервера

Этот пакет демонстрирует реализацию высокопроизводительного **Broadcast WebSocket-сервера** на базе библиотеки `hioload-ws`. Сервер работает на нативном TCP-слое без использования стандартного `net/http`, обеспечивая нулевое копирование данных, lock-free пакетную (batch) обработку и NUMA-aware привязку потоков для экстремальной производительности и минимальной латентности.

## Содержание

1. [Обзор](#%D0%BE%D0%B1%D0%B7%D0%BE%D1%80)
2. [Ключевые возможности](#%D0%BA%D0%BB%D1%8E%D1%87%D0%B5%D0%B2%D1%8B%D0%B5-%D0%B2%D0%BE%D0%B7%D0%BC%D0%BE%D0%B6%D0%BD%D0%BE%D1%81%D1%82%D0%B8)
3. [Требования](#%D1%82%D1%80%D0%B5%D0%B1%D0%BE%D0%B2%D0%B0%D0%BD%D0%B8%D1%8F)
4. [Установка](#%D1%83%D1%81%D1%82%D0%B0%D0%BD%D0%BE%D0%B2%D0%BA%D0%B0)
5. [Конфигурация](#%D0%BA%D0%BE%D0%BD%D1%84%D0%B8%D0%B3%D1%83%D1%80%D0%B0%D1%86%D0%B8%D1%8F)
6. [Архитектура](#%D0%B0%D1%80%D1%85%D0%B8%D1%82%D0%B5%D0%BA%D1%82%D1%83%D1%80%D0%B0)
6.1 [Паттерн фасада](#%D0%BF%D0%B0%D1%82%D1%82%D0%B5%D1%80%D0%BD-%D1%84%D0%B0%D1%81%D0%B0%D0%B4%D0%B0)
6.2 [Цепочка middleware](#%D1%86%D0%B5%D0%BF%D0%BE%D1%87%D0%BA%D0%B0-middleware)
6.3 [Zero-copy буферный пул](#zero-copy-%D0%B1%D1%83%D1%84%D0%B5%D1%80%D0%BD%D1%8B%D0%B9-%D0%BF%D1%83%D0%BB)
6.4 [Пакетный reactor (poll-mode)](#%D0%BF%D0%B0%D0%BA%D0%B5%D1%82%D0%BD%D1%8B%D0%B9-reactor-poll-mode)
6.5 [NUMA-aware привязка](#numa-aware-%D0%BF%D1%80%D0%B8%D0%B2%D1%8F%D0%B7%D0%BA%D0%B0)
6.6 [Lock-free структуры данных](#lock-free-%D1%81%D1%82%D1%80%D1%83%D0%BA%D1%82%D1%83%D1%80%D1%8B-%D0%B4%D0%B0%D0%BD%D0%BD%D1%8B%D1%85)
7. [Разбор кода](#%D1%80%D0%B0%D0%B7%D0%B1%D0%BE%D1%80-%D0%BA%D0%BE%D0%B4%D0%B0)
7.1 [main.go](#maingo)
7.2 [go.mod](#gomod)
8. [Запуск примера](#%D0%B7%D0%B0%D0%BF%D1%83%D1%81%D0%BA-%D0%BF%D1%80%D0%B8%D0%BC%D0%B5%D1%80%D0%B0)
9. [Тюнинг производительности](#%D1%82%D1%8E%D0%BD%D0%B8%D0%BD%D0%B3-%D0%BF%D1%80%D0%BE%D0%B8%D0%B7%D0%B2%D0%BE%D0%B4%D0%B8%D1%82%D0%B5%D0%BB%D1%8C%D0%BD%D0%BE%D1%81%D1%82%D0%B8)
10. [Отладка и метрики](#%D0%BE%D1%82%D0%BB%D0%B0%D0%B4%D0%BA%D0%B0-%D0%B8-%D0%BC%D0%B5%D1%82%D1%80%D0%B8%D0%BA%D0%B8)
11. [Корректный shutdown](#%D0%BA%D0%BE%D1%80%D1%80%D0%B5%D0%BA%D1%82%D0%BD%D1%8B%D0%B9-shutdown)
12. [Расширение примера](#%D1%80%D0%B0%D1%81%D1%88%D0%B8%D1%80%D0%B5%D0%BD%D0%B8%D0%B5-%D0%BF%D1%80%D0%B8%D0%BC%D0%B5%D1%80%D0%B0)
13. [Лицензия](#%D0%BB%D0%B8%D1%86%D0%B5%D0%BD%D0%B7%D0%B8%D1%8F)
14. [Вклад и развитие](#%D0%B2%D0%BA%D0%BB%D0%B0%D0%B4-%D0%B8-%D1%80%D0%B0%D0%B7%D0%B2%D0%B8%D1%82%D0%B8%D0%B5)

## Обзор

**Broadcast Server** принимает бинарные фреймы от клиентов и транслирует их всем остальным. Пример оптимизирован для экстремальных нагрузок:

- Отказ от одного горутины на соединение в пользу общего **пакетного реактора**.
- Обслуживание миллионов соединений с p99-латентностью на уровне микросекунд.
- NUMA-aware аллокация zero-copy буферов и lock-free очереди для минимизации переключений контекста и GC-нагрузки.
- Привязка reactor и executor потоков к критическим CPU/NUMA узлам для максимальной локальности.


## Ключевые возможности

- **Zero-Copy**: буферы выделяются из NUMA-aware пула, без лишних копий.
- **Batch Poll-Mode I/O**: обработка до N событий за цикл.
- **NUMA-aware привязка**: потоки и аллокации буферов закрепляются за NUMA-узлами.
- **Lock-Free структуры**: кольцевые буферы и лок-фри очереди.
- **Middleware Chain**: легко расширяемые цепочки логирования, восстановления и метрики.
- **Graceful Shutdown**: корректная остановка на SIGINT/SIGTERM.
- **Кроссплатформенность**: Linux (epoll, io_uring) и Windows (IOCP).


## Требования

- Go 1.23+
- Linux ≥6.20 или Windows Server 2016+ (amd64)
- Модуль `github.com/momentics/hioload-ws`
- Знания Go и системного программирования


## Установка

```bash
git clone https://github.com/momentics/hioload-ws.git
cd hioload-ws/examples/lowlevel/broadcast
go mod tidy
```


## Конфигурация

Флаги командной строки:


| Флаг | По умолчанию | Описание |
| :-- | :-- | :-- |
| `-addr` | `:9002` | Адрес для прослушивания WebSocket |
| `-batch` | `64` | Количество событий на итерацию реактора |
| `-ring` | `2048` | Ёмкость кольцевого буфера реактора |
| `-workers` | `0` | Число воркеров (0 = runtime.NumCPU()) |
| `-numa` | `-1` | Предпочтительный NUMA-узел для буферов и пининга (-1 = авто) |

Пример:

```bash
go run main.go -addr=":9002" -batch=128 -ring=4096 -workers=8 -numa=0
```


## Архитектура

### Паттерн фасада

Пакет `/server` предоставляет фасад:

1. **ControlAdapter**: динамические настройки, метрики, debug-пробы.
2. **BufferPoolManager**: NUMA-aware zero-copy буферы.
3. **WebSocketListener**: native TCP + handshake.
4. **PollerAdapter**: batch-mode реактор.
5. **ExecutorAdapter**: lock-free NUMA-aware исполнитель задач.

Инициализация:

```go
cfg := server.DefaultConfig()
cfg.ListenAddr = *addr
cfg.BatchSize = *batch
cfg.ReactorRing = *ring
cfg.ExecutorWorkers = *workers
cfg.NUMANode = *numa

srv, err := server.NewServer(cfg,
    server.WithMiddleware(adapters.LoggingMiddleware),
    server.WithMiddleware(adapters.RecoveryMiddleware),
)
```


### Цепочка middleware

Middleware-обёртки для кросс-сечений:

```go
srv.UseMiddleware(
    adapters.LoggingMiddleware,
    adapters.RecoveryMiddleware,
    trackingMiddleware,
)
```


### Zero-copy буферный пул

Пулы размер-классов, сегментированные по NUMA-узлам, без heap-аллокаций.

### Пакетный reactor (poll-mode)

`PollerAdapter` настроен на пакетную обработку до `BatchSize` событий, без mutex-ов в горячей зоне.

### NUMA-aware привязка

В момент запуска reactor-поток закрепляется:

```go
aff := adapters.NewAffinityAdapter()
aff.Pin(-1, cfg.NUMANode)
defer aff.Unpin()
```


### Lock-free структуры данных

- **RingBuffer**: concurrent FIFO.
- **LockFreeQueue**: SPSC задача-очередь.
- **SlabPool**: lock-free стек.


## Разбор кода

### main.go

```go
package main

import (
    "flag"
    "fmt"
    "os"
    "os/signal"
    "sync"
    "sync/atomic"
    "syscall"
    "time"

    "github.com/momentics/hioload-ws/adapters"
    "github.com/momentics/hioload-ws/api"
    "github.com/momentics/hioload-ws/core/protocol"
    "github.com/momentics/hioload-ws/lowlevel/server"
)

func main() {
    // Флаги
    addr := flag.String("addr", ":9002", "адрес WebSocket")
    batch := flag.Int("batch", 64, "batch size")
    ring := flag.Int("ring", 2048, "ring capacity")
    workers := flag.Int("workers", 0, "число воркеров")
    numa := flag.Int("numa", -1, "NUMA uzel")
    flag.Parse()

    // Конфиг сервера
    cfg := server.DefaultConfig()
    cfg.ListenAddr = *addr
    cfg.BatchSize = *batch
    cfg.ReactorRing = *ring
    if *workers > 0 {
        cfg.ExecutorWorkers = *workers
    }
    cfg.NUMANode = *numa

    srv, err := server.NewServer(cfg,
        server.WithMiddleware(adapters.LoggingMiddleware),
        server.WithMiddleware(adapters.RecoveryMiddleware),
    )
    if err != nil {
        fmt.Fprintf(os.Stderr, "NewServer error: %v\n", err)
        os.Exit(1)
    }

    // Реестр соединений
    var (
        conns     = make(map[*protocol.WSConnection]struct{})
        connsLock sync.RWMutex
        totalMsgs int64
    )

    // Debug-пробы
    ctrl := srv.GetControl()
    ctrl.RegisterDebugProbe("connections", func() any {
        connsLock.RLock()
        n := len(conns)
        connsLock.RUnlock()
        return n
    })
    ctrl.RegisterDebugProbe("total_messages", func() any {
        return atomic.LoadInt64(&totalMsgs)
    })

    // Тикер статистики
    go func() {
        ticker := time.NewTicker(2 * time.Second)
        defer ticker.Stop()
        for range ticker.C {
            stats := ctrl.Stats()
            fmt.Printf("[%s] Conns: %v, Msgs: %v\n",
                time.Now().Format(time.Stamp),
                stats["connections"],
                stats["total_messages"],
            )
        }
    }()

    // Broadcast-хендлер
    handler := adapters.HandlerFunc(func(data any) error {
        buf := data.(api.Buffer)
        defer buf.Release()
        payload := buf.Bytes()
        atomic.AddInt64(&totalMsgs, 1)

        conn := api.FromContext(api.ContextFromData(data)).(*protocol.WSConnection)
        bcast := make([]byte, len(payload))
        copy(bcast, payload)

        connsLock.RLock()
        for ws := range conns {
            if ws != conn {
                frame := &protocol.WSFrame{
                    IsFinal:    true,
                    Opcode:     protocol.OpcodeBinary,
                    PayloadLen: int64(len(bcast)),
                    Payload:    bcast,
                }
                ws.SendFrame(frame)
            }
        }
        connsLock.RUnlock()
        return nil
    })

    // Middleware учёта соединений
    track := func(next api.Handler) api.Handler {
        return adapters.HandlerFunc(func(data any) error {
            switch evt := data.(type) {
            case api.OpenEvent:
                ws := evt.Conn.(*protocol.WSConnection)
                connsLock.Lock()
                conns[ws] = struct{}{}
                connsLock.Unlock()
            case api.CloseEvent:
                ws := evt.Conn.(*protocol.WSConnection)
                connsLock.Lock()
                delete(conns, ws)
                connsLock.Unlock()
            }
            return next.Handle(data)
        })
    }
    srv.UseMiddleware(track)

    // Запуск
    go func() {
        if err := srv.Run(handler); err != nil {
            fmt.Fprintf(os.Stderr, "Run error: %v\n", err)
            os.Exit(1)
        }
    }()

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    fmt.Println("Shutting down broadcast server...")
    srv.Shutdown()
    fmt.Println("Server stopped.")
}
```


### go.mod

```go
module github.com/momentics/hioload-ws/examples/lowlevel/broadcast

go 1.23

require github.com/momentics/hioload-ws v0.0.0
```


## Запуск примера

```bash
cd examples/lowlevel/broadcast
go build -o broadcast
./broadcast -addr=":9002" -batch=64 -ring=2048 -workers=8 -numa=0
```

Подключитесь через `wscat`:

```bash
wscat -c ws://localhost:9002
```

Любое сообщение будет передано всем остальным подключённым клиентам.

## Тюнинг производительности

- **Batch Size**: увеличение повышает пропускную способность, снижает задержку.
- **Ring Capacity**: настройка под пики нагрузки.
- **Executor Workers**: должно соответствовать числу ядер на NUMA-узел.
- **NUMA Pinning**: используйте флаг `-numa` для оптимальной привязки.


## Отладка и метрики

- Регистрация дополнительных debug-проб:

```go
srv.GetControl().RegisterDebugProbe("my_probe", func() any { return someValue })
```

- Экспорт метрик через HTTP или интеграцию с Prometheus.
- Профилирование с помощью `pprof` и трассировки.


## Корректный shutdown

При SIGINT/SIGTERM сервер:

1. Останавливает приём новых подключений.
2. Останавливает реактор и обработчики задач.
3. Дожидается завершения in-flight событий в пределах таймаута.
4. Закрывает транспорт и освобождает ресурсы.

## Лицензия

Apache 2.0 — см. [LICENSE](../../LICENSE).

## Контакты и вклад в проект

- **Автор**: momentics <momentics@gmail.com>  
- **Репозиторий**: https://github.com/momentics/hioload-ws  
- Вопросы, багрепорты и pull request’ы приветствуются!
