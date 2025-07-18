# reactor

Реактор событий, используемый в high-load WebSocket-библиотеке hioload-ws. Поддерживает кроссплатформенную архитектуру:

- **Linux**: использует механизм `epoll`
- **Windows**: использует механизм `IOCP (I/O Completion Ports)`

Полностью реализован на языке Go с применением golang.org/x/sys/{unix,windows}, без стандартной syscall-библиотеки.

---

## Архитектура

### Интерфейс (platform-neutral)
`reactor.go`

- Содержит определения интерфейса `EventReactor` и структуры события `Event`.
- Не содержит платформонезависимой логики и ссылок на platform-specific реализации.

### Реализации (platform-specific)

| Платформа | Файл                   | Технология |
|-----------|------------------------|-------------|
| Linux     | `reactor_linux.go`     | `epoll`     |
| Windows   | `reactor_windows.go`   | `IOCP`      |
| Другие    | `reactor_stub.go`      | stub (ошибка при использовании) |

Каждая реализация содержит собственную версию конструктора `NewReactor`, экспортируемого по тегу сборки, и реализует интерфейс `EventReactor`.

---

## EventReactor

```

type EventReactor interface {
Register(fd uintptr, userData uintptr) error
Wait(events []Event) (int, error)
Close() error
}

```

### Register(fd, userData)

Регистрирует файловый дескриптор (Linux) или дескриптор HANDLE (Windows) в реакторе. `userData` позволяет сохранить произвольное число, связанное с дескриптором (например, указатель на соединение).

### Wait(events)

Ожидание новых событий. Ожидаемые события заполняются в переданный слайс `[]Event`. Метод блокирует выполнение, пока не появятся события.

### Close()

Освобождает ресурсы. Закрывает epoll-дескриптор или хендл IOCP.

---

## Структура Event

```

type Event struct {
Fd       uintptr // файловый дескриптор или HANDLE
UserData uintptr // значение, переданное при регистрации
}

```

---

## Пример использования

```

package main

import (
"fmt"
"log"
"hioload-ws/reactor"
)

func main() {
r, err := reactor.NewReactor()
if err != nil {
log.Fatalf("Не удалось создать реактор: %v", err)
}
defer r.Close()

    // пример регистрации fd/handle с userData
    // err := r.Register(fd, uintptr(unsafe.Pointer(conn)))
    // ...
    
    events := make([]reactor.Event, 128)
    n, err := r.Wait(events)
    if err != nil {
        log.Fatalf("Ошибка при ожидании событий: %v", err)
    }
    
    for i := 0; i < n; i++ {
        fmt.Printf("Событие: fd=%d, userData=0x%x\n", events[i].Fd, events[i].UserData)
    }
    }

```

---

## Завершение

Этот модуль представляет собой низкоуровневую платформонезависимую абстракцию над системными примитивами обработки событий. Он активно используется в net-цикле hioload-ws и поддерживает многопоточную масштабируемую архитектуру с минимальными накладными расходами.

Для расширения на другие платформы (например, BSD/kqueue или Linux/io_uring) необходимо просто реализовать `NewReactor()` с использованием соответствующих механик и тегов сборки.

---

Разработчик: [momentics](mailto:momentics@gmail.com)
