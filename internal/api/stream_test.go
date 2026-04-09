package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestStreamLog(t *testing.T) {
	var gotRequest streamRequest
	var mu sync.Mutex
	var ready sync.WaitGroup
	ready.Add(1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read handshake: %v", err)
			return
		}
		mu.Lock()
		json.Unmarshal(msg, &gotRequest)
		mu.Unlock()
		ready.Done()

		conn.WriteMessage(websocket.TextMessage, []byte("log line 1\n"))
		conn.WriteMessage(websocket.TextMessage, []byte("log line 2\n"))
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}))
	defer srv.Close()

	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1)
	c := &Client{baseURL: wsURL, tenant: "test", doer: srv.Client()}

	streamer, err := c.StreamLog("uuid-123", "console.log")
	if err != nil {
		t.Fatal(err)
	}
	defer streamer.Close()

	ready.Wait()
	mu.Lock()
	if gotRequest.UUID != "uuid-123" {
		t.Errorf("UUID = %q", gotRequest.UUID)
	}
	if gotRequest.Logfile != "console.log" {
		t.Errorf("Logfile = %q", gotRequest.Logfile)
	}
	mu.Unlock()

	msg1, err := streamer.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg1 != "log line 1\n" {
		t.Errorf("msg1 = %q", msg1)
	}

	msg2, err := streamer.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg2 != "log line 2\n" {
		t.Errorf("msg2 = %q", msg2)
	}
}

func TestStreamLog_DefaultLogfile(t *testing.T) {
	var gotLogfile string
	var ready sync.WaitGroup
	ready.Add(1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		_, msg, _ := conn.ReadMessage()
		var req streamRequest
		json.Unmarshal(msg, &req)
		gotLogfile = req.Logfile
		ready.Done()

		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}))
	defer srv.Close()

	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1)
	c := &Client{baseURL: wsURL, tenant: "t", doer: srv.Client()}

	streamer, err := c.StreamLog("uuid", "")
	if err != nil {
		t.Fatal(err)
	}
	defer streamer.Close()

	ready.Wait()
	if gotLogfile != "console.log" {
		t.Errorf("expected default logfile, got %q", gotLogfile)
	}
}

func TestLogStreamer_Close_Nil(t *testing.T) {
	s := &LogStreamer{conn: nil}
	if err := s.Close(); err != nil {
		t.Errorf("Close on nil conn should return nil, got %v", err)
	}
}
