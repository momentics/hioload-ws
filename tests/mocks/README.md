# Mocks for hioload-ws

Mocks in this directory help to thoroughly unit- and integration-test
high-load critical paths, including concurrency/ring/executor/protocol flows.

- `FakeExecutor.go`       — trivial implementation of Executor
- `protocol_stub.go`      — protocol stubs for roundtrip/fault injection
- `transport_mock.go`     — see base set, covers Transport/Features
- `control_mock.go`       — see base set, covers Control/Config/Stats
