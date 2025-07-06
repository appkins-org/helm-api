package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/appkins-org/helm-api/internal/template"
	"github.com/gorilla/schema"
)

var decoder = schema.NewDecoder()

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/template", handleTemplate)

	log.Printf("Server starting on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"message": "Helm API Server",
		"version": "1.0.0",
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status": "healthy",
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters into TemplateRequest
	req, err := parseTemplateRequest(r)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		errorResponse := map[string]string{"error": err.Error()}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(errorResponse)
		return
	}

	result, err := template.ProcessTemplate(req)
	if err != nil {
		// Check if it's a validation error (400) vs processing error (500)
		if strings.Contains(err.Error(), "chart is required") ||
			strings.Contains(err.Error(), "invalid") {
			w.Header().Set("Content-Type", "application/json")
			errorResponse := map[string]string{"error": err.Error()}
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(errorResponse)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		errorResponse := map[string]string{"error": err.Error()}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(errorResponse)
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml")
	_, _ = w.Write([]byte(result))
}

// parseTemplateRequest parses HTTP query parameters into a TemplateRequest.
func parseTemplateRequest(r *http.Request) (template.TemplateRequest, error) {
	query := r.URL.Query()

	req := template.TemplateRequest{}

	// First try to decode with the schema decoder
	err := decoder.Decode(&req, query)
	if err != nil {
		return req, err
	}
	return req, nil
}
