// Package main
// Author: momentics <momentics@gmail.com>
//
// Echo server example using hioload-ws facade and correct pool usage.

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/momentics/hioload-ws/adapters"
	"github.com/momentics/hioload-ws/facade"
	"github.com/momentics/hioload-ws/protocol"
)

type EchoHandler struct {
	hioload *facade.HioloadWS
}

func (e *EchoHandler) Handle(data any) error {
	switch msg := data.(type) {
	case []byte:
		log.Printf("Received: %s", string(msg))
		buf := e.hioload.GetBuffer(len(msg))
		defer buf.Release()
		copy(buf.Bytes(), msg)
		return e.hioload.GetTransport().Send([][]byte{buf.Bytes()})
	default:
		return fmt.Errorf("unsupported type %T", data)
	}
}

func main() {
	config := facade.DefaultConfig()
	config.NumWorkers = 4
	hioload, _ := facade.New(config)
	hioload.Start()
	defer hioload.Stop()

	handler := &EchoHandler{hioload: hioload}
	mw := adapters.NewMiddlewareHandler(handler).
		Use(adapters.LoggingMiddleware).
		Use(adapters.RecoveryMiddleware).
		Use(adapters.MetricsMiddleware(hioload.GetControl()))
	hioload.RegisterHandler(mw)

	upgrader := protocol.Upgrader{} // uses websocket.Upgrader under the hood
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		headers, err := protocol.UpgradeToWebSocket(r)
		if err != nil {
			http.Error(w, "upgrade failed", 400)
			return
		}
		for k, v := range headers {
			w.Header()[k] = v
		}
		w.WriteHeader(101)
		conn := hioload.CreateWebSocketConnection()
		conn.Start()
	})

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
