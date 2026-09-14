package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Config for the local Anthropic→OpenAI bridge.
type Config struct {
	// UpstreamBase is the OpenAI-compatible root ending in /v1 (e.g. Runpod …/openai/v1).
	UpstreamBase string
	// UpstreamKey is forwarded as Authorization: Bearer to the upstream.
	UpstreamKey string
	// DefaultModel overrides Claude-ish model ids in requests.
	DefaultModel string
	// ListenAddr e.g. "127.0.0.1:0" or "127.0.0.1:9090".
	ListenAddr string
	// ExpectedToken optional; if set, require Bearer or x-api-key to match.
	ExpectedToken string
}

// Server is a local HTTP bridge.
type Server struct {
	cfg    Config
	client *http.Client
	http   *http.Server
	ln     net.Listener
	mu     sync.Mutex
}

// Start listens and serves in the background. Returns the bound base URL
// (http://127.0.0.1:<port>) with no trailing path — Claude uses this as ANTHROPIC_BASE_URL.
func Start(cfg Config) (*Server, string, error) {
	cfg.UpstreamBase = strings.TrimRight(strings.TrimSpace(cfg.UpstreamBase), "/")
	if cfg.UpstreamBase == "" {
		return nil, "", fmt.Errorf("bridge: empty upstream base")
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:0"
	}
	s := &Server{
		cfg: cfg,
		client: &http.Client{
			Timeout: 10 * time.Minute,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				MaxIdleConns:          32,
				IdleConnTimeout:       90 * time.Second,
				ResponseHeaderTimeout: 5 * time.Minute,
			},
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/messages", s.handleMessages)
	mux.HandleFunc("/v1/messages/", s.handleMessages) // count_tokens under prefix
	mux.HandleFunc("/v1/models", s.handleModels)
	mux.HandleFunc("/", s.handleRoot)

	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return nil, "", err
	}
	s.ln = ln
	s.http = &http.Server{Handler: mux, ReadHeaderTimeout: 15 * time.Second}
	go func() { _ = s.http.Serve(ln) }()
	addr := ln.Addr().String()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		_ = s.Close()
		return nil, "", err
	}
	if host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	base := fmt.Sprintf("http://%s:%s", host, port)
	return s, base, nil
}

// Close shuts down the listener.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if s.http != nil {
		_ = s.http.Shutdown(ctx)
	}
	if s.ln != nil {
		return s.ln.Close()
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "bridge": "anthropic-openai"})
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || r.URL.Path == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"name":     "runhug-anthropic-bridge",
			"upstream": s.cfg.UpstreamBase,
			"endpoints": []string{
				"POST /v1/messages",
				"POST /v1/messages/count_tokens",
				"GET /v1/models",
				"GET /health",
			},
		})
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := s.cfg.DefaultModel
	if id == "" {
		id = "default"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data": []map[string]any{
			{"id": id, "object": "model", "owned_by": "runhug"},
		},
	})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	if strings.HasSuffix(path, "/count_tokens") {
		s.handleCountTokens(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorize(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write(anthropicErrorJSON("authentication_error", "invalid api key"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		anthropicHTTPError(w, http.StatusBadRequest, "invalid_request_error", "failed to read body")
		return
	}
	_ = r.Body.Close()

	openaiBody, areq, err := TranslateRequest(body, s.cfg.DefaultModel)
	if err != nil {
		anthropicHTTPError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	modelForResp := areq.Model
	if dm := strings.TrimSpace(s.cfg.DefaultModel); dm != "" && (modelForResp == "" || isClaudeish(modelForResp)) {
		modelForResp = dm
	}

	upRes, err := s.doChatCompletions(r.Context(), openaiBody)
	if err != nil {
		anthropicHTTPError(w, http.StatusBadGateway, "api_error", "upstream: "+err.Error())
		return
	}
	// worker-vllm often 500s on tools unless ENABLE_AUTO_TOOL_CHOICE + TOOL_CALL_PARSER
	// are configured (and some models still lack tool support). Retry once without tools.
	if upRes.StatusCode >= 500 && openaiBodyHasTools(openaiBody) {
		_ = upRes.Body.Close()
		stripped := stripTools(openaiBody)
		fmt.Fprintln(os.Stderr, "runhug bridge: upstream rejected tools (HTTP 5xx); retrying without tools/tool_choice")
		upRes, err = s.doChatCompletions(r.Context(), stripped)
		if err != nil {
			anthropicHTTPError(w, http.StatusBadGateway, "api_error", "upstream: "+err.Error())
			return
		}
	}
	defer upRes.Body.Close()

	if upRes.StatusCode < 200 || upRes.StatusCode >= 300 {
		ub, _ := io.ReadAll(io.LimitReader(upRes.Body, 1<<20))
		msg := strings.TrimSpace(string(ub))
		if len(msg) > 800 {
			msg = msg[:800] + "…"
		}
		typ := "api_error"
		if upRes.StatusCode == 401 || upRes.StatusCode == 403 {
			typ = "authentication_error"
		}
		anthropicHTTPError(w, upRes.StatusCode, typ, fmt.Sprintf("upstream HTTP %d: %s", upRes.StatusCode, msg))
		return
	}

	ct := upRes.Header.Get("Content-Type")
	if areq.Stream || strings.Contains(ct, "text/event-stream") {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		bw := &flushWriter{w: w, f: flusher}
		if err := PipeOpenAISSE(upRes.Body, bw, modelForResp); err != nil {
			// Best-effort; headers already sent.
			return
		}
		return
	}

	ub, err := io.ReadAll(io.LimitReader(upRes.Body, 16<<20))
	if err != nil {
		anthropicHTTPError(w, http.StatusBadGateway, "api_error", "read upstream: "+err.Error())
		return
	}
	out, err := TranslateResponse(ub, modelForResp)
	if err != nil {
		anthropicHTTPError(w, http.StatusBadGateway, "api_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

func (s *Server) handleCountTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		anthropicHTTPError(w, http.StatusBadRequest, "invalid_request_error", "failed to read body")
		return
	}
	var req anthropicReq
	if err := json.Unmarshal(body, &req); err != nil {
		anthropicHTTPError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"input_tokens": EstimateTokens(req)})
}

func (s *Server) authorize(r *http.Request) bool {
	want := strings.TrimSpace(s.cfg.ExpectedToken)
	if want == "" {
		return true
	}
	got := ""
	if ah := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(ah), "bearer ") {
		got = strings.TrimSpace(ah[7:])
	}
	if got == "" {
		got = strings.TrimSpace(r.Header.Get("x-api-key"))
	}
	return got == want
}

type flushWriter struct {
	w io.Writer
	f http.Flusher
}

func (f *flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if f.f != nil {
		f.f.Flush()
	}
	return n, err
}

func (f *flushWriter) Flush() {
	if f.f != nil {
		f.f.Flush()
	}
}

func (s *Server) doChatCompletions(ctx context.Context, openaiBody []byte) (*http.Response, error) {
	upURL := s.cfg.UpstreamBase + "/chat/completions"
	upReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upURL, bytes.NewReader(openaiBody))
	if err != nil {
		return nil, err
	}
	upReq.Header.Set("Content-Type", "application/json")
	if key := strings.TrimSpace(s.cfg.UpstreamKey); key != "" {
		upReq.Header.Set("Authorization", "Bearer "+key)
	}
	return s.client.Do(upReq)
}

// stripTools removes tools and tool_choice from an OpenAI chat-completions JSON body.
func stripTools(openaiBody []byte) []byte {
	var m map[string]any
	if err := json.Unmarshal(openaiBody, &m); err != nil {
		return openaiBody
	}
	delete(m, "tools")
	delete(m, "tool_choice")
	delete(m, "parallel_tool_calls")
	out, err := json.Marshal(m)
	if err != nil {
		return openaiBody
	}
	return out
}

func openaiBodyHasTools(body []byte) bool {
	var probe struct {
		Tools json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return false
	}
	raw := bytes.TrimSpace(probe.Tools)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) || bytes.Equal(raw, []byte("[]")) {
		return false
	}
	return true
}

func anthropicHTTPError(w http.ResponseWriter, status int, typ, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(anthropicErrorJSON(typ, msg))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
