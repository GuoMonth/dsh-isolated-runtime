// Command dshprobe exercises the exact Phase 1 DSH protocol surface without a
// browser. It intentionally stores the authority-bound cookie only in a
// caller-provided mode-0600 state file.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type probeState struct {
	Cookie    string `json:"cookie"`
	SessionID string `json:"sessionId"`
}

type rpcResponse[T any] struct {
	Result struct {
		OK    bool `json:"ok"`
		Value T    `json:"value"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	} `json:"result"`
}

func main() {
	connect := flag.String("connect", "", "TCP address reached by the probe, for example 127.0.0.1:18080")
	authority := flag.String("authority", "", "external Host authority preserved by the launcher")
	stateFile := flag.String("state-file", "", "mode-0600 file used to persist cookie and session id across a restart")
	resume := flag.Bool("resume", false, "reuse existing state without performing a launch-token exchange")
	flag.Parse()
	if err := run(*connect, *authority, *stateFile, *resume); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "dshprobe: %v\n", err)
		os.Exit(1)
	}
}

func run(connect, authority, stateFile string, resume bool) error {
	if connect == "" || authority == "" || stateFile == "" {
		return errors.New("--connect, --authority and --state-file are required")
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp", connect)
	}}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	base := "http://" + authority

	state := probeState{}
	if resume {
		value, err := os.ReadFile(stateFile)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(value, &state); err != nil {
			return err
		}
		if state.Cookie == "" || state.SessionID == "" {
			return errors.New("probe state is incomplete")
		}
	} else {
		request, err := http.NewRequest(http.MethodGet, base+"/", nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err != nil {
			return fmt.Errorf("cookie exchange: %w", err)
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusSeeOther {
			return fmt.Errorf("cookie exchange status = %d", response.StatusCode)
		}
		if strings.Contains(response.Header.Get("Location"), "token=") {
			return errors.New("launch token escaped into redirect")
		}
		cookies := response.Cookies()
		if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
			return fmt.Errorf("unexpected DSH cookie attributes")
		}
		state.Cookie = cookies[0].Name + "=" + cookies[0].Value
	}

	if _, err := rpc[map[string]any](client, base, authority, state.Cookie, "settings/describe", map[string]any{}); err != nil {
		return err
	}
	if !resume {
		created, err := rpc[struct {
			SessionID string `json:"sessionId"`
		}](client, base, authority, state.Cookie, "session/create", map[string]any{"request": map[string]any{}})
		if err != nil {
			return err
		}
		if created.SessionID == "" {
			return errors.New("session/create returned no id")
		}
		state.SessionID = created.SessionID
	}
	listed, err := rpc[struct {
		Items []struct {
			SessionID string `json:"sessionId"`
		} `json:"items"`
	}](client, base, authority, state.Cookie, "session/list", map[string]any{"_request": map[string]any{}})
	if err != nil {
		return err
	}
	found := false
	for _, item := range listed.Items {
		found = found || item.SessionID == state.SessionID
	}
	if !found {
		return errors.New("durable session was not listed")
	}
	if err := followSnapshot(connect, authority, state); err != nil {
		return err
	}
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		request, err := http.NewRequest(method, base+"/api/session.export?sessionId="+url.QueryEscape(state.SessionID), nil)
		if err != nil {
			return err
		}
		request.Header.Set("Cookie", state.Cookie)
		response, err := client.Do(request)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", method, err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("fetch %s status = %d", method, response.StatusCode)
		}
	}
	if !resume {
		encoded, err := json.Marshal(state)
		if err != nil {
			return err
		}
		if err := os.WriteFile(stateFile, encoded, 0o600); err != nil {
			return err
		}
		if err := os.Chmod(stateFile, 0o600); err != nil {
			return err
		}
	}
	fmt.Printf("DSH protocol probe passed (resume=%t)\n", resume)
	return nil
}

func rpc[T any](
	client *http.Client,
	base, authority, cookie, method string,
	args map[string]any,
) (T, error) {
	var zero T
	payload, err := json.Marshal(map[string]any{
		"type":    "client-request",
		"rpcId":   "phase1-" + strings.ReplaceAll(method, "/", "-"),
		"method":  method,
		"payload": map[string]any{"args": args},
	})
	if err != nil {
		return zero, err
	}
	request, err := http.NewRequest(http.MethodPost, base+"/api/"+method, bytes.NewReader(payload))
	if err != nil {
		return zero, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cookie", cookie)
	request.Header.Set("Origin", "https://"+authority)
	response, err := client.Do(request)
	if err != nil {
		return zero, fmt.Errorf("%s: %w", method, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
		return zero, fmt.Errorf("%s status = %d: %s", method, response.StatusCode, strings.TrimSpace(string(body)))
	}
	var body rpcResponse[T]
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return zero, err
	}
	if !body.Result.OK {
		return zero, fmt.Errorf("%s failed: %s: %s", method, body.Result.Error.Code, body.Result.Error.Message)
	}
	return body.Result.Value, nil
}

func followSnapshot(connect, authority string, state probeState) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", connect)
		},
	}
	headers := http.Header{
		"Cookie": []string{state.Cookie},
		"Origin": []string{"https://" + authority},
	}
	connection, response, err := dialer.Dial("ws://"+authority+"/api/remote.mux", headers)
	if err != nil {
		if response != nil {
			_ = response.Body.Close()
			return fmt.Errorf("remote.mux websocket status = %d: %w", response.StatusCode, err)
		}
		return fmt.Errorf("remote.mux websocket: %w", err)
	}
	defer func() { _ = connection.Close() }()
	streamID := "phase1-follow"
	if err := connection.WriteJSON(map[string]any{
		"type":     "open",
		"streamId": streamID,
		"endpoint": "session/follow",
		"payload": map[string]any{"args": map[string]any{
			"request": map[string]any{"address": map[string]any{"kind": "session", "sessionId": state.SessionID}},
		}},
	}); err != nil {
		return err
	}
	_ = connection.SetReadDeadline(time.Now().Add(15 * time.Second))
	for {
		var frame map[string]any
		if err := connection.ReadJSON(&frame); err != nil {
			return err
		}
		if frame["streamId"] != streamID {
			continue
		}
		if frame["type"] == "error" || frame["type"] == "end" {
			return fmt.Errorf("session/follow ended before a snapshot: %#v", frame)
		}
		value, _ := frame["value"].(map[string]any)
		if frame["type"] == "item" && value["type"] == "snapshot" {
			return nil
		}
	}
}
