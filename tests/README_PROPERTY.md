# Property-based and Integration Testing in hioload-ws

## Property-based Tests

`property_ring_test.go` — validates invariants of ring buffers using randomized actions.
- Checks for never overflow/underflow.
- Ensures atomicity and correctness under hundreds/thousands of random ops.

## Integration Tests

`integration_echo_test.go` — spins up a full echo server and verifies WS flow using Gorilla WebSocket.
- Validates real ws handshake, message flow, and echo correctness.

## Heavy Concurrency

`concurrency_deadlock_test.go` — simulates massive parallel create/get/delete in SessionManager.
- Ensures no deadlocks and all session logic is thread-safe under stress.

## Performance/Load

`benchmark_transport_parallel_test.go` — microbenchmarks batch send/recv and buffer pooling in concurrent hot path.
- Run: `go test -bench . ./tests/...`

`loadtest_k6_sample.js` — use with [k6](https://k6.io/) for high-level scenario stress/load-testing:
- Opens thousands of WS connections, sends messages, checks echo responses.

---

**All files licensed under Apache 2.0, authored by momentics@gmail.com.**


Инструкция по запуску
Модульные/интеграционные/Concurrency:
go test -race ./tests/...

Property-based:
go test -run Property ./tests/property_ring_test.go

Перфоманс-бенчмарки:
go test -bench . ./tests/benchmark_transport_parallel_test.go

K6 нагрузочное тестирование:
k6 run tests/loadtest_k6_sample.js