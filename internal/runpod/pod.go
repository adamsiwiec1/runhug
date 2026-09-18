package runpod

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Pod lifecycle statuses as reported by RunPod v2.
const (
	PodStatusProvisioning = "PROVISIONING"
	PodStatusStarting     = "STARTING"
	PodStatusRunning      = "RUNNING"
	PodStatusExited       = "EXITED"
	PodStatusError        = "ERROR"
	PodStatusTerminated   = "TERMINATED"
)

// Pod actions are the state transitions accepted by POST /v2/pods/{id}/action.
const (
	PodActionStart     = "start"
	PodActionStop      = "stop"
	PodActionRestart   = "restart"
	PodActionTerminate = "terminate"
)

// DefaultHereticImage is the runhug heretic training dashboard container.
const DefaultHereticImage = "ghcr.io/adamsiwiec1/runhug-heretic:latest"

// DefaultDashboardPort is the HTTP port the heretic container exposes.
const DefaultDashboardPort = 8080

// Pod is the ReadPod/GetPod response shape from RunPod v2.
type Pod struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Status       string      `json:"status"`
	Actions      []string    `json:"actions"`
	GPU          *PodGPU     `json:"gpu"`
	Cloud        string      `json:"cloud"`
	DataCenterID any         `json:"dataCenterId"`
	Cost         *float64    `json:"cost"`
	Runtime      *PodRuntime `json:"runtime"`
	CreatedAt    string      `json:"createdAt"`
	StartedAt    string      `json:"startedAt"`
}

// PodGPU mirrors CreateGpuConfig and is also echoed back by getPod.
type PodGPU struct {
	ID             string   `json:"id"`
	Count          int      `json:"count,omitempty"`
	MinCudaVersion string   `json:"minCudaVersion,omitempty"`
	AllowedCuda    []string `json:"allowedCudaVersions,omitempty"`
	MinRamPerGpu   int      `json:"minRamPerGpu,omitempty"`
	MinVcpuPerGpu  int      `json:"minVcpuCountPerGpu,omitempty"`
}

// PodRuntime carries live utilization and port mapping for a running pod.
type PodRuntime struct {
	Uptime int              `json:"uptime"`
	GPUs   []PodGPUUtil     `json:"gpus"`
	CPU    *Utilization     `json:"cpu"`
	Memory *Utilization     `json:"memory"`
	Ports  []PodRuntimePort `json:"ports"`
}

type PodGPUUtil struct {
	Utilization float64 `json:"utilization"`
	MemoryUsed  float64 `json:"memoryUsed"`
}

type Utilization struct {
	Percent     float64 `json:"percent"`
	Used        float64 `json:"used"`
	Total       float64 `json:"total"`
	BytesPerSec float64 `json:"bytesPerSecond,omitempty"`
}

type PodRuntimePort struct {
	Private int    `json:"private"`
	Public  *int   `json:"public"`
	Type    string `json:"type"`
	IP      string `json:"ip,omitempty"`
}

type CreatePodRequest struct {
	Name          string            `json:"name"`
	Image         string            `json:"image,omitempty"`
	Registry      string            `json:"registry,omitempty"`
	Args          string            `json:"args,omitempty"`
	Disk          int               `json:"disk,omitempty"`
	Ports         []string          `json:"ports,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	GPU           *PodGPU           `json:"gpu,omitempty"`
	Cloud         string            `json:"cloud,omitempty"`
	DataCenterIDs []string          `json:"dataCenterIds,omitempty"`
	StartJupyter  bool              `json:"startJupyter,omitempty"`
	StartSSH      bool              `json:"startSsh,omitempty"`
}

// RegistryCredential is a stored container-registry credential (registries API).
type RegistryCredential struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *Client) ListRegistryCredentials(ctx context.Context) ([]RegistryCredential, error) {
	var wrap struct {
		Data       []RegistryCredential `json:"data"`
		Items      []RegistryCredential `json:"items"`
		Registries []RegistryCredential `json:"registries"`
	}
	if err := c.do(ctx, http.MethodGet, "/v2/registries", nil, &wrap); err != nil {
		return nil, err
	}
	if len(wrap.Registries) > 0 {
		return wrap.Registries, nil
	}
	if len(wrap.Data) > 0 {
		return wrap.Data, nil
	}
	return wrap.Items, nil
}

func (c *Client) CreateRegistryCredential(ctx context.Context, name, username, password string) (*RegistryCredential, error) {
	req := map[string]string{"name": name, "username": username, "password": password}
	var wrap struct {
		Data *RegistryCredential `json:"data"`
		ID   string              `json:"id"`
		Name string              `json:"name"`
	}
	if err := c.do(ctx, http.MethodPost, "/v2/registries", req, &wrap); err != nil {
		return nil, err
	}
	if wrap.Data != nil {
		return wrap.Data, nil
	}
	return &RegistryCredential{ID: wrap.ID, Name: wrap.Name}, nil
}

// RegistryCredentialFor returns the ID of an existing credential with name,
// creating it first if absent.
func (c *Client) RegistryCredentialFor(ctx context.Context, name, username, password string) (string, error) {
	existing, err := c.ListRegistryCredentials(ctx)
	if err != nil {
		return "", err
	}
	for _, cred := range existing {
		if cred.Name == name && cred.ID != "" {
			return cred.ID, nil
		}
	}
	created, err := c.CreateRegistryCredential(ctx, name, username, password)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func (c *Client) ListPods(ctx context.Context) ([]Pod, error) {
	var wrap struct {
		Pods  []Pod `json:"pods"`
		Items []Pod `json:"items"`
	}
	if err := c.do(ctx, http.MethodGet, "/v2/pods", nil, &wrap); err != nil {
		return nil, err
	}
	if len(wrap.Pods) > 0 {
		return wrap.Pods, nil
	}
	return wrap.Items, nil
}

func (c *Client) CreatePod(ctx context.Context, req CreatePodRequest) (*Pod, error) {
	var pod Pod
	if err := c.do(ctx, http.MethodPost, "/v2/pods", req, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

func (c *Client) GetPod(ctx context.Context, id string) (*Pod, error) {
	var pod Pod
	if err := c.do(ctx, http.MethodGet, "/v2/pods/"+url.PathEscape(id), nil, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

// PodAction triggers a state transition (start | stop | restart | terminate).
func (c *Client) PodAction(ctx context.Context, id, action string) (*Pod, error) {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case PodActionStart, PodActionStop, PodActionRestart, PodActionTerminate:
	default:
		return nil, fmt.Errorf("runpod: unsupported pod action %q", action)
	}
	var pod Pod
	req := map[string]string{"action": strings.ToLower(strings.TrimSpace(action))}
	if err := c.do(ctx, http.MethodPost, "/v2/pods/"+url.PathEscape(id)+"/action", req, &pod); err != nil {
		return nil, err
	}
	return &pod, nil
}

// DeletePod terminates a pod (DELETE /v2/pods/{id}).
func (c *Client) DeletePod(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v2/pods/"+url.PathEscape(id), nil, nil)
}

// Terminal reports whether the pod status means it is done or dead.
func (p *Pod) Terminal() bool {
	switch strings.ToUpper(strings.TrimSpace(p.Status)) {
	case PodStatusExited, PodStatusError, PodStatusTerminated:
		return true
	}
	return false
}

// Running reports whether the pod container is healthy.
func (p *Pod) Running() bool {
	return strings.EqualFold(strings.TrimSpace(p.Status), PodStatusRunning)
}

// PodProxyURL returns the RunPod HTTP proxy URL for an exposed pod port.
// Format: https://<pod-id>-<port>.proxy.runpod.net
func PodProxyURL(podID string, port int) string {
	if strings.TrimSpace(podID) == "" {
		return ""
	}
	if port <= 0 {
		port = DefaultDashboardPort
	}
	return "https://" + podID + "-" + strconv.Itoa(port) + ".proxy.runpod.net"
}

// IsOutOfStock reports whether a RunPod error was a transient no-stock
// rejection ("no longer any instances available ..."), which a caller can
// retry against the next-cheapest card.
func IsOutOfStock(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no longer any instances") ||
		strings.Contains(msg, "no instances available") ||
		strings.Contains(msg, "out of stock")
}

// PodHourlyPrice returns the pay-as-you-go pod price for a GPU catalog entry.
// SECURE pods bill at Price.Secure; COMMUNITY pods at Price.Community.
func (g GPU) PodHourlyPrice() float64 {
	if g.Price.Secure > 0 {
		return g.Price.Secure
	}
	if g.Price.Community > 0 {
		return g.Price.Community
	}
	return g.ServerlessPrice()
}
