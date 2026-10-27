package panel

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var liveUpgrader = websocket.Upgrader{
	HandshakeTimeout: 5 * time.Second, ReadBufferSize: 1024, WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") != "" && sameOrigin(r) },
}

func (p *Panel) live(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, 405, "method not allowed")
		return
	}
	if r.Header.Get("Origin") == "" || !sameOrigin(r) {
		fail(w, 403, "origin rejected")
		return
	}
	frames, initial, ok := p.Live.subscribe()
	if !ok {
		fail(w, 503, "live dashboard connection limit; retry shortly")
		return
	}
	defer p.Live.unsubscribe(frames)
	conn, err := liveUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(50 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(50 * time.Second)) })
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		// Dashboard is receive-only. Any application message ends the stream.
		_, _, _ = conn.ReadMessage()
		_ = conn.Close()
	}()
	write := func(frame []byte) error {
		_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		return conn.WriteMessage(websocket.TextMessage, frame)
	}
	if err := write(initial); err != nil {
		return
	}
	auth := time.NewTicker(time.Second)
	defer auth.Stop()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-ended:
			return
		case <-r.Context().Done():
			return
		case <-p.Live.done:
			return
		case <-auth.C:
			if _, _, valid := p.session(r); !valid {
				_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4001, "session expired"), time.Now().Add(time.Second))
				return
			}
		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second)); err != nil {
				return
			}
		case frame := <-frames:
			if _, _, valid := p.session(r); !valid {
				return
			}
			if err := write(frame); err != nil {
				return
			}
		}
	}
}
