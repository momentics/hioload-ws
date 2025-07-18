# Пакет /api/

Пакет `/api` представляет собой слой платформонезависимых интерфейсов, определяющих архитектурные контракты между основными подсистемами библиотеки hioload-ws.  
Он не содержит реализаций, а служит базой для всех модулей: `reactor`, `transport`, `pool`, `protocol`, `websocket`.

Интерфейсы написаны с учётом требований к сверхвысоконагруженным системам:  
- нулевая аллокация (zero-copy)
- NUMA-осведомлённость
- низкие накладные расходы
- кроссплатформенность (Linux, Windows)
- читаемость и тестируемость (интерфейсы легко подменяются в unit/integration тестах)

---

## Структура

| Файл             | Описание                                                         |
|------------------|------------------------------------------------------------------|
| `reactor.go`     | Интерфейс реактора событий и структура Event для epoll/IOCP     |
| `transport.go`   | Абстракция над сетевым соединением (`NetConn`), вместо `net.Conn` |
| `pool.go`        | Интерфейсы пулы байтов и объектов (`BytePool`, `ObjectPool`)     |
| `websocket.go`   | WebSocket-соединение (`WebSocketConn`) и фреймы (`WebSocketFrame`) |

---

## Основные интерфейсы

### Reactor

```

type Reactor interface {
Register(fd uintptr, userData uintptr) error
Wait(events []Event) (int, error)
Close() error
}

```

Абстракция над системами оповещения о событиях ввода-вывода (epoll, IOCP, etc).  
Используется в слое сетевого стека для масштабируемости.

---

### NetConn

```

type NetConn interface {
Read(p []byte) (int, error)
Write(p []byte) (int, error)
Close() error
RawFD() uintptr
}

```

Низкоуровневая транспортная абстракция сети. Позволяет использовать неблокирующие дескрипторы, zero-copy-схемы, адаптацию к reactor.

---

### BytePool и ObjectPool

```

type BytePool interface {
Acquire(n int) []byte
Release(buf []byte)
}

type ObjectPool[T any] interface {
Get() T
Put(obj T)
}

```

Важные строительные блоки высокоэффективной среды:
- BytePool — используется строками ввода/вывода и декодерами
- ObjectPool — для повторного использования структур

Поддерживают NUMA-aware реализацию, large pages, sync.Pool и другие backends

---

### WebSocketConn и WebSocketFrame

```

type WebSocketConn interface {
ReadFrame() (WebSocketFrame, error)
WriteFrame(fin bool, opcode byte, payload []byte) error
CloseStream() error
}

```

Контракт frame-based протокольного слоя поверх транспорта. Упрощает создание и расширение WebSocket-серверов/прокси/роутеров.

---

## Назначение

Пакет `/api` позволяет:

- Стандартизировать API между слоями
- Упростить тестирование и мокирование
- Исключить циклические зависимости
- Облегчить расширение архитектуры (например: HTTP/2, custom pools, io_uring и др.)

