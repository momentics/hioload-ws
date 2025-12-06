// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Error definitions for concurrency module.

package concurrency

import "errors"

var (
	// ErrExecutorClosed indicates the executor has been shut down
	ErrExecutorClosed = errors.New("executor is closed")

	// ErrTaskTimeout indicates a task exceeded its timeout
	ErrTaskTimeout = errors.New("task execution timeout")

	// ErrInvalidWorkerCount indicates invalid worker count configuration
	ErrInvalidWorkerCount = errors.New("invalid worker count")

	// ErrAffinityNotSupported indicates CPU affinity is not supported on this platform
	ErrAffinityNotSupported = errors.New("CPU affinity not supported")

	// ErrNUMANotAvailable indicates NUMA is not available
	ErrNUMANotAvailable = errors.New("NUMA not available")
)
