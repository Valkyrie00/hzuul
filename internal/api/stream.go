package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
)

// LogStreamer reads log messages from a Zuul console-stream WebSocket.
type LogStreamer struct {
	conn *websocket.Conn
}

// streamRequest is the JSON payload sent to the console-stream WebSocket,
// matching the format used by Zuul's web UI (Stream.jsx).
type streamRequest struct {
	UUID    string `json:"uuid"`
	Logfile string `json:"logfile,omitempty"`
}

// StreamLog opens a WebSocket to Zuul's console-stream endpoint and sends
// the handshake message in the JSON format the server expects.
func (c *Client) StreamLog(uuid, logfile string) (*LogStreamer, error) {
	wsURL := c.baseURL + c.tenantPath("console-stream")
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)

	dialer := websocket.Dialer{}

	if hc, ok := c.doer.(*http.Client); ok {
		if t, ok := hc.Transport.(*http.Transport); ok && t.TLSClientConfig != nil {
			dialer.TLSClientConfig = t.TLSClientConfig.Clone()
		}
	}

	headers := http.Header{}
	if hc, ok := c.doer.(*http.Client); ok && hc.Jar != nil {
		u, _ := url.Parse(c.baseURL)
		if u != nil {
			var parts []string
			for _, cookie := range hc.Jar.Cookies(u) {
				parts = append(parts, cookie.Name+"="+cookie.Value)
			}
			if len(parts) > 0 {
				headers.Set("Cookie", strings.Join(parts, "; "))
			}
		}
	}

	conn, _, err := dialer.Dial(wsURL, headers)
	if err != nil {
		return nil, fmt.Errorf("connecting to console-stream: %w", err)
	}

	// Zuul streams the entire accumulated log as the first message(s),
	// which can be very large for long-running jobs.
	conn.SetReadLimit(10 << 20) // 10 MB

	if logfile == "" {
		logfile = "console.log"
	}
	msg, _ := json.Marshal(streamRequest{UUID: uuid, Logfile: logfile})
	if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("sending stream request: %w", err)
	}

	return &LogStreamer{conn: conn}, nil
}

func (s *LogStreamer) ReadMessage() (string, error) {
	_, msg, err := s.conn.ReadMessage()
	if err != nil {
		return "", err
	}
	return string(msg), nil
}

func (s *LogStreamer) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
