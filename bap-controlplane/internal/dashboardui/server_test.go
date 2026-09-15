package dashboardui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDashboardServesAssetsAndProxiesAPI(t *testing.T) {
	var expectedHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != expectedHost {
			t.Errorf("proxy did not rewrite host: %q", r.Host)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"upstream"}`))
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	expectedHost = target.Host
	handler := Handler(target, http.DefaultTransport, false)

	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest("GET", "/dashboard/", nil))
	if page.Code != 200 || !strings.Contains(page.Body.String(), "Agent operations") {
		t.Fatalf("dashboard asset response: %d", page.Code)
	}

	api := httptest.NewRecorder()
	handler.ServeHTTP(api, httptest.NewRequest("GET", "/api/v1/health", nil))
	if api.Code != 200 || !strings.Contains(api.Body.String(), "upstream") {
		t.Fatalf("proxied API response: %d %s", api.Code, api.Body.String())
	}

	config := httptest.NewRecorder()
	handler.ServeHTTP(config, httptest.NewRequest("GET", "/dashboard-config", nil))
	if config.Code != 200 || !strings.Contains(config.Body.String(), upstream.URL) {
		t.Fatalf("dashboard config response: %d %s", config.Code, config.Body.String())
	}

	inspector := httptest.NewRecorder()
	handler.ServeHTTP(inspector, httptest.NewRequest("GET", "/inspector", nil))
	if inspector.Code != 200 || !strings.Contains(inspector.Body.String(), "upstream") {
		t.Fatalf("proxied inspector response: %d %s", inspector.Code, inspector.Body.String())
	}

	inspectorV2 := httptest.NewRecorder()
	handler.ServeHTTP(inspectorV2, httptest.NewRequest("GET", "/inspector_v2", nil))
	if inspectorV2.Code != 200 || !strings.Contains(inspectorV2.Body.String(), "upstream") {
		t.Fatalf("proxied inspector_v2 response: %d %s", inspectorV2.Code, inspectorV2.Body.String())
	}
}

func TestRemoteAdministrativeProxyIsDeniedByDefault(t *testing.T) {
	target, _ := url.Parse("http://127.0.0.1:1")
	handler := Handler(target, http.DefaultTransport, false)
	r := httptest.NewRequest("POST", "/api/v1/control/kill-switch", strings.NewReader(`{"enabled":true}`))
	r.RemoteAddr = "203.0.113.9:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("remote admin request status %d, want 403", w.Code)
	}
}
