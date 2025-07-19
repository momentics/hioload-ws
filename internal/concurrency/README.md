# internal/concurrency/

Этот модуль реализует высокоэффективный слой конкурентности и событийного планирования для hioload-ws. Архитектура — NUMA-aware, zero-copy-friendly, lock-free и кроссплатформенная (Linux/Windows).

- Включает thread pool с динамическим масштабированием, affinity/NUMA pinning, и ultra-low-latency scheduler.
- Используются только передовые паттерны высокого уровня для DPDK, ZC, reactor-подобной обработки.
- Платформенные детали абстрагированы через build tags (см. pin.go, pin_linux.go, pin_windows.go).
- Абсолютно все интерфейсы реализуют best practices, предназначены для потоковых, событийных и таймерных сценариев транспорта и WebSocket-протокола.

Подробнее см. комментарии в исходном коде.
