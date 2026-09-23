// ws-hold checks that revocation terminates an already established native stream.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	statePath := flag.String("state", "", "private dshprobe state file")
	connect := flag.String("connect", "127.0.0.1:30500", "test proxy address")
	authority := flag.String("authority", "sandbox-spike.cells.test", "bound origin authority")
	flag.Parse()
	data, err := os.ReadFile(*statePath)
	if err != nil {
		panic(err)
	}
	var state struct {
		Cookie    string
		SessionID string
	}
	if err = json.Unmarshal(data, &state); err != nil {
		panic(err)
	}
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second, NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", *connect)
	}}
	conn, _, err := dialer.Dial("ws://"+*authority+"/api/remote.mux", http.Header{"Origin": {"https://" + *authority}, "Cookie": {state.Cookie}})
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	if err = conn.WriteJSON(map[string]any{"type": "open", "streamId": "hold", "endpoint": "session/follow", "payload": map[string]any{"args": map[string]any{"request": map[string]any{"address": map[string]any{"kind": "session", "sessionId": state.SessionID}}}}}); err != nil {
		panic(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, _, err = conn.ReadMessage()
	if err != nil {
		panic(err)
	}
	fmt.Println("stream-open")
	for {
		_, _, err = conn.ReadMessage()
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Timeout() {
				panic("revocation did not close stream")
			}
			fmt.Println("stream-closed")
			return
		}
	}
}
