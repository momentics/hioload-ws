// File: api/shutdown.go
// Package api defines unified graceful shutdown contract.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package api

// GracefulShutdown объединяет логику корректного завершения компонентов.
type GracefulShutdown interface {
	// Shutdown выполняет корректную остановку всех внутренних сервисов
	// и освобождение ресурсов. Возвращает ошибку при неудаче.
	Shutdown() error
}
