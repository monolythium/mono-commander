package mesh

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHealthChecker(t *testing.T) {
	hc := NewHealthChecker()

	if hc.Client == nil {
		t.Error("NewHealthChecker() Client should not be nil")
	}

	if hc.Timeout != 5*time.Second {
		t.Errorf("NewHealthChecker() Timeout = %v, want 5s", hc.Timeout)
	}
}

func TestHealthChecker_Check_HealthEndpoint(t *testing.T) {
	// Create test server with /health endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	hc := NewHealthChecker()
	ctx := context.Background()

	status := hc.Check(ctx, server.URL)

	if !status.Healthy {
		t.Error("Check() should return Healthy=true for working health endpoint")
	}

	if status.Method != "health_endpoint" {
		t.Errorf("Check() Method = %s, want health_endpoint", status.Method)
	}

	if status.ResponseTime < 0 {
		t.Error("Check() ResponseTime should be non-negative")
	}
}

func TestHealthChecker_Check_NetworkList(t *testing.T) {
	// Create test server with /network/list endpoint (Rosetta standard)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/network/list" && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"network_identifiers": []map[string]string{
					{"blockchain": "monolythium", "network": "sprintnet"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	hc := NewHealthChecker()
	ctx := context.Background()

	status := hc.Check(ctx, server.URL)

	if !status.Healthy {
		t.Error("Check() should return Healthy=true for working network/list endpoint")
	}

	if status.Method != "network_list" {
		t.Errorf("Check() Method = %s, want network_list", status.Method)
	}
}

func TestHealthChecker_Check_TCPFallback(t *testing.T) {
	// Create test server that doesn't respond to /health or /network/list
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	hc := NewHealthChecker()
	ctx := context.Background()

	status := hc.Check(ctx, server.URL)

	// Should still be healthy via TCP port check
	if !status.Healthy {
		t.Error("Check() should return Healthy=true when TCP port is open")
	}

	if status.Method != "tcp_port" {
		t.Errorf("Check() Method = %s, want tcp_port", status.Method)
	}
}

func TestHealthChecker_Check_Unhealthy(t *testing.T) {
	hc := NewHealthChecker()
	ctx := context.Background()

	// Check against an address that doesn't exist
	status := hc.Check(ctx, "127.0.0.1:59999")

	if status.Healthy {
		t.Error("Check() should return Healthy=false for non-responding address")
	}

	if status.Error == "" {
		t.Error("Check() should set Error for failed check")
	}
}

func TestNormalizeAddressToURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://localhost:8080", "http://localhost:8080"},
		{"https://example.com/", "https://example.com"},
		{"0.0.0.0:8080", "http://127.0.0.1:8080"},
		{"127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"localhost:8080", "http://localhost:8080"},
		{"8080", "http://127.0.0.1:8080"},
		{"example.com:8080", "http://example.com:8080"},
	}

	for _, tt := range tests {
		got := normalizeAddressToURL(tt.input)
		if got != tt.want {
			t.Errorf("normalizeAddressToURL(%s) = %s, want %s", tt.input, got, tt.want)
		}
	}
}

func TestHealthChecker_CheckWithConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &Config{
		ListenAddress: server.URL[7:], // Strip "http://"
	}

	hc := NewHealthChecker()
	ctx := context.Background()

	status := hc.CheckWithConfig(ctx, cfg)

	if !status.Healthy {
		t.Error("CheckWithConfig() should return Healthy=true for working service")
	}
}

func TestHealthStatus_JSON(t *testing.T) {
	status := &HealthStatus{
		Healthy:       true,
		Method:        "health_endpoint",
		ListenAddress: "0.0.0.0:8080",
		ResponseTime:  42,
		Details:       map[string]interface{}{"status": "ok"},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded HealthStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Healthy != status.Healthy {
		t.Error("JSON round-trip failed for Healthy")
	}

	if decoded.Method != status.Method {
		t.Error("JSON round-trip failed for Method")
	}
}
