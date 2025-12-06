// Package protocol
// Author: momentics <momentics@gmail.com>
//
// Implements the core WebSocket protocol logic (RFC 6455) for hioload-ws.
//
// Designed for ultra-high-load message processing environments using zero-copy,
// NUMA-aware buffers, lock-free decode structures, and fully streaming-safe decoding.
//
// Includes:
//   - Frame encoding/decoding over pooled buffers
//   - Ping/Pong/Close control frame FSM
//   - Full masking support per spec (browser client compliance)
//   - Platform-independent logic and hooks into transport layer
//   - Memory-safe, reuse-oriented parsers
package protocol
