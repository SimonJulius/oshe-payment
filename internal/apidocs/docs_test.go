package apidocs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocumentationRoutes(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)
	for _, path := range []string{"/docs", "/openapi.yaml"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d", path, w.Code)
		}
		if path == "/docs" && !strings.Contains(w.Body.String(), "url:'/openapi.yaml'") {
			t.Fatal("UI must load local contract")
		}
	}
	var contract struct {
		OpenAPI string         `json:"openapi"`
		Paths   map[string]any `json:"paths"`
	}
	// JSON is also valid YAML; keep the embedded contract parseable without dependencies.
	if err := json.Unmarshal(spec, &contract); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/health/live", "/health/ready", "/testing/notifications", "/testing/notifications/{id}", "/webhooks/brevo"} {
		if contract.Paths[path] == nil {
			t.Fatalf("missing path %s", path)
		}
	}
	// Documentation registration must not enable privileged testing routes.
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/testing/notifications", nil))
	if w.Code != 404 {
		t.Fatalf("testing route enabled by docs: %d", w.Code)
	}
}
