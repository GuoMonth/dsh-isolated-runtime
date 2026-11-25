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
	"hash/fnv"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type probeState struct {
	Cookie    string `json:"cookie"`
	SessionID string `json:"sessionId"`
}

type sessionRecord struct {
	SessionID string `json:"sessionId"`
}

type rpcArguments struct {
	Request     *struct{} `json:"request,omitempty"`
	ListRequest *struct{} `json:"_request,omitempty"`
}

type rpcPayload struct {
	Args rpcArguments `json:"args"`
}

type rpcRequest struct {
	Type    string     `json:"type"`
	RPCID   string     `json:"rpcId"`
	Method  string     `json:"method"`
	Payload rpcPayload `json:"payload"`
}

type rpcResponse[T any] struct {
	Result struct {
		OK    bool `json:"ok"`
		Value T    `json:"value"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	} `json:"result"`
}

type sessionAddress struct {
	Kind      string `json:"kind"`
	SessionID string `json:"sessionId"`
}

type followRequest struct {
	Address sessionAddress `json:"address"`
}

type followFrame struct {
	Type     string `json:"type"`
	StreamID string `json:"streamId"`
	Value    struct {
		Type string `json:"type"`
	} `json:"value"`
}

func main() {
	policyUID := flag.String("envoy-cookie-suffix-for-uid", "", "print the pinned Gateway's unpadded FNV-1a cookie suffix for a fixture policy UID")
	connect := flag.String("connect", "", "TCP address reached by the probe, for example 127.0.0.1:18080")
	authority := flag.String("authority", "", "external Host authority preserved by the launcher")
	stateFile := flag.String("state-file", "", "mode-0600 file used to persist cookie and session id across a restart")
	resume := flag.Bool("resume", false, "reuse existing state without performing a launch-token exchange")
	flag.Parse()
	if *policyUID != "" {
		hash := fnv.New32a()
		_, _ = hash.Write([]byte(*policyUID))
		fmt.Printf("%x\n", hash.Sum32())
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	if err := run(ctx, *connect, *authority, *stateFile, *resume); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "dshprobe: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, connect, authority, stateFile string, resume bool) error {
	if connect == "" || authority == "" || stateFile == "" {
		return errors.New("--connect, --authority and --state-file are required")
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp", connect)
	}}
	defer transport.CloseIdleConnections()
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
			return fmt.Errorf("read probe state: %w", err)
		}
		if err := json.Unmarshal(value, &state); err != nil {
			return fmt.Errorf("decode probe state: %w", err)
		}
		if state.Cookie == "" || state.SessionID == "" {
			return errors.New("probe state is incomplete")
		}
	} else {
		cookie, err := exchangeCookie(ctx, client, base)
		if err != nil {
			return err
		}
		state.Cookie = cookie
	}

	if _, err := rpc[json.RawMessage](ctx, client, base, authority, state.Cookie, "settings/describe", rpcArguments{}); err != nil {
		return err
	}
	if !resume {
		created, err := rpc[sessionRecord](ctx, client, base, authority, state.Cookie, "session/create", rpcArguments{Request: &struct{}{}})
		if err != nil {
			return err
		}
		if created.SessionID == "" {
			return errors.New("session/create returned no id")
		}
		state.SessionID = created.SessionID
	}
	listed, err := rpc[struct {
		Items []sessionRecord `json:"items"`
	}](ctx, client, base, authority, state.Cookie, "session/list", rpcArguments{ListRequest: &struct{}{}})
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(listed.Items, func(item sessionRecord) bool { return item.SessionID == state.SessionID }) {
		return errors.New("durable session was not listed")
	}
	if err := followSnapshot(ctx, connect, authority, state); err != nil {
		return err
	}
	if err := verifyExports(ctx, client, base, state); err != nil {
		return err
	}
	if !resume {
		encoded, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("encode probe state: %w", err)
		}
		if err := os.WriteFile(stateFile, encoded, 0o600); err != nil {
			return fmt.Errorf("write probe state: %w", err)
		}
		if err := os.Chmod(stateFile, 0o600); err != nil {
			return fmt.Errorf("restrict probe state permissions: %w", err)
		}
	}
	if _, err := fmt.Printf("DSH protocol probe passed (resume=%t)\n", resume); err != nil {
		return fmt.Errorf("write probe result: %w", err)
	}
	return nil
}

func exchangeCookie(ctx context.Context, client *http.Client, base string) (cookie string, resultErr error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/", nil)
	if err != nil {
		return "", fmt.Errorf("build cookie exchange request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("cookie exchange: %w", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close cookie exchange response: %w", err))
		}
	}()
	if response.StatusCode != http.StatusSeeOther {
		return "", fmt.Errorf("cookie exchange status = %d", response.StatusCode)
	}
	if strings.Contains(response.Header.Get("Location"), "token=") {
		return "", errors.New("launch token escaped into redirect")
	}
	cookies := response.Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		return "", errors.New("unexpected DSH cookie attributes")
	}
	return cookies[0].Name + "=" + cookies[0].Value, nil
}

func verifyExports(ctx context.Context, client *http.Client, base string, state probeState) error {
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		request, err := http.NewRequestWithContext(ctx, method, base+"/api/session.export?sessionId="+url.QueryEscape(state.SessionID), nil)
		if err != nil {
			return fmt.Errorf("build %s export request: %w", method, err)
		}
		request.Header.Set("Cookie", state.Cookie)
		response, err := client.Do(request)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", method, err)
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return fmt.Errorf("read %s export: %w", method, err)
		}
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("fetch %s status = %d", method, response.StatusCode)
		}
	}
	return nil
}

func rpc[T any](ctx context.Context, client *http.Client, base, authority, cookie, method string, args rpcArguments) (value T, resultErr error) {
	payload, err := json.Marshal(rpcRequest{
		Type:    "client-request",
		RPCID:   "phase1-" + strings.ReplaceAll(method, "/", "-"),
		Method:  method,
		Payload: rpcPayload{Args: args},
	})
	if err != nil {
		return value, fmt.Errorf("encode %s request: %w", method, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/"+method, bytes.NewReader(payload))
	if err != nil {
		return value, fmt.Errorf("build %s request: %w", method, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cookie", cookie)
	request.Header.Set("Origin", "https://"+authority)
	response, err := client.Do(request)
	if err != nil {
		return value, fmt.Errorf("%s: %w", method, err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close %s response: %w", method, err))
		}
	}()
	if response.StatusCode != http.StatusOK {
		return value, fmt.Errorf("%s status = %d", method, response.StatusCode)
	}
	var body rpcResponse[T]
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return value, fmt.Errorf("decode %s response: %w", method, err)
	}
	if !body.Result.OK {
		return value, fmt.Errorf("%s failed: %s", method, body.Result.Error.Code)
	}
	return body.Result.Value, nil
}

func followSnapshot(ctx context.Context, connect, authority string, state probeState) (resultErr error) {
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
	connection, response, err := dialer.DialContext(ctx, "ws://"+authority+"/api/remote.mux", headers)
	if err != nil {
		if response != nil {
			return fmt.Errorf("remote.mux websocket status = %d: %w", response.StatusCode, errors.Join(err, response.Body.Close()))
		}
		return fmt.Errorf("remote.mux websocket: %w", err)
	}
	closed := make(chan error, 1)
	stop := context.AfterFunc(ctx, func() { closed <- connection.Close() })
	defer func() {
		var err error
		if stop() {
			err = connection.Close()
		} else {
			err = <-closed
		}
		if err != nil && !errors.Is(err, net.ErrClosed) {
			resultErr = errors.Join(resultErr, fmt.Errorf("close session/follow connection: %w", err))
		}
	}()

	const streamID = "phase1-follow"
	message := struct {
		Type     string `json:"type"`
		StreamID string `json:"streamId"`
		Endpoint string `json:"endpoint"`
		Payload  struct {
			Args struct {
				Request followRequest `json:"request"`
			} `json:"args"`
		} `json:"payload"`
	}{Type: "open", StreamID: streamID, Endpoint: "session/follow"}
	message.Payload.Args.Request.Address = sessionAddress{Kind: "session", SessionID: state.SessionID}
	if err := connection.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return fmt.Errorf("set session/follow write deadline: %w", err)
	}
	if err := connection.WriteJSON(message); err != nil {
		return fmt.Errorf("open session/follow: %w", err)
	}
	if err := connection.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return fmt.Errorf("set session/follow read deadline: %w", err)
	}
	for {
		var frame followFrame
		if err := connection.ReadJSON(&frame); err != nil {
			return fmt.Errorf("read session/follow: %w", errors.Join(ctx.Err(), err))
		}
		if frame.StreamID != streamID {
			continue
		}
		switch frame.Type {
		case "error", "end":
			return fmt.Errorf("session/follow returned %s before a snapshot", frame.Type)
		case "item":
			if frame.Value.Type == "snapshot" {
				return nil
			}
		}
	}
}
