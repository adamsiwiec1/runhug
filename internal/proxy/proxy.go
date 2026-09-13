package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/adamsiwiec/runpod-vllm-proxy/internal/config"
	"github.com/adamsiwiec/runpod-vllm-proxy/internal/runpod"
	"github.com/adamsiwiec/runpod-vllm-proxy/internal/store"
)

type Server struct {
	Registry *store.Registry
	APIKey   string
}

func New(reg *store.Registry, apiKey string) *Server {
	return &Server{Registry: reg, APIKey: config.SanitizeAPIKey(apiKey)}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/v1/models", s.models)
	mux.HandleFunc("/openai/v1/models", s.models)
	mux.HandleFunc("/v1/", s.forward)
	mux.HandleFunc("/openai/v1/", s.forward)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"current": s.Registry.Current,
		"models":  len(s.Registry.Models),
	})
}

func (s *Server) models(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	type item struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	}
	data := make([]item, 0, len(s.Registry.Models))
	for id := range s.Registry.Models {
		data = append(data, item{ID: id, Object: "model", OwnedBy: "runpod-vllm-proxy"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (s *Server) forward(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && (strings.HasSuffix(r.URL.Path, "/models") || r.URL.Path == "/v1/models" || r.URL.Path == "/openai/v1/models") {
		s.models(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		openaiError(w, http.StatusBadRequest, "failed to read body")
		return
	}
	_ = r.Body.Close()

	model := ""
	var peek struct {
		Model string `json:"model"`
	}
	if len(body) > 0 && json.Unmarshal(body, &peek) == nil {
		model = peek.Model
	}
	entry, ok := s.Registry.Lookup(model)
	if !ok {
		openaiError(w, http.StatusNotFound, "unknown model "+model+"; register it with init, local add, or deploy")
		return
	}
	if rewritten, err := rewriteModel(body, entry.UpstreamModel()); err == nil {
		body = rewritten
	}

	suffix := openaiSuffix(r.URL.Path)
	upstream := runpod.OpenAIURL(entry.EndpointID)
	if entry.Kind() == store.BackendLocal {
		if entry.BaseURL == "" {
			openaiError(w, http.StatusBadGateway, "local model has no base_url; run `local start`")
			return
		}
		upstream = strings.TrimRight(entry.BaseURL, "/")
	}
	target, err := url.Parse(upstream + suffix)
	if err != nil {
		openaiError(w, http.StatusBadGateway, "bad upstream url")
		return
	}
	if r.URL.RawQuery != "" {
		target.RawQuery = r.URL.RawQuery
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			// SetURL joins inbound and target paths; we already built the
			// full upstream path (local /v1/... or Runpod /v2/.../openai/v1/...).
			u := *target
			pr.Out.URL = &u
			pr.Out.Host = u.Host
			pr.SetXForwarded()
			if entry.Kind() == store.BackendRunpod {
				if key := config.SanitizeAPIKey(s.APIKey); key != "" {
					pr.Out.Header.Set("Authorization", "Bearer "+key)
				}
			}
			pr.Out.Header.Del("Cookie")
		},
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			openaiError(w, http.StatusBadGateway, "upstream: "+err.Error())
		},
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Set("Content-Length", itoa(len(body)))
	proxy.ServeHTTP(w, r)
}

func rewriteModel(body []byte, id string) ([]byte, error) {
	if len(body) == 0 || id == "" {
		return body, nil
	}
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return body, err
	}
	obj["model"] = id
	return json.Marshal(obj)
}

func openaiSuffix(path string) string {
	path = strings.TrimPrefix(path, "/openai")
	path = strings.TrimPrefix(path, "/v1")
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func openaiError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": msg,
			"type":    "invalid_request_error",
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func ListenAndServe(addr string, h http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 10 * time.Second}
	return srv.ListenAndServe()
}
