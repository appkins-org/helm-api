package health

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
)

func TestHealthEndpoint(t *testing.T) {
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := New(logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))))
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("health handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"status":"healthy"}`
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("health handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

// BenchmarkHealthHandler benchmarks the health endpoint.
func BenchmarkHealthHandler(b *testing.B) {
	req, _ := http.NewRequest("GET", "/health", nil)

	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler := New(logr.FromContextAsSlogLogger(logr.NewContext(b.Context(), testr.NewWithInterface(b, testr.Options{}))))
		handler.ServeHTTP(rr, req)
	}
}
