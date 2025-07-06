package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/appkins-org/helm-api/internal/template"
)

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
	json.NewEncoder(w).Encode(response)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status": "healthy",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
		json.NewEncoder(w).Encode(errorResponse)
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
			json.NewEncoder(w).Encode(errorResponse)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		errorResponse := map[string]string{"error": err.Error()}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse)
		return
	}

	w.Header().Set("Content-Type", "application/x-yaml")
	w.Write([]byte(result))
}

// parseTemplateRequest parses HTTP query parameters into a TemplateRequest
func parseTemplateRequest(r *http.Request) (template.TemplateRequest, error) {
	query := r.URL.Query()

	req := template.TemplateRequest{}

	// Required parameter: chart
	chart := query.Get("chart")
	if chart == "" {
		return req, fmt.Errorf("chart parameter is required")
	}
	req.Chart = chart

	// Optional parameters
	if releaseName := query.Get("release_name"); releaseName != "" {
		req.ReleaseName = releaseName
	}

	if namespace := query.Get("namespace"); namespace != "" {
		req.Namespace = namespace
	}

	if kubeVersion := query.Get("kube_version"); kubeVersion != "" {
		req.KubeVersion = kubeVersion
	}

	if chartVersion := query.Get("chart_version"); chartVersion != "" {
		req.ChartVersion = chartVersion
	}

	if repository := query.Get("repository"); repository != "" {
		req.Repository = repository
	}

	// Boolean parameters
	if includeCRDs := query.Get("include_crds"); includeCRDs == "true" {
		req.IncludeCRDs = true
	}

	if skipTests := query.Get("skip_tests"); skipTests == "true" {
		req.SkipTests = true
	}

	if isUpgrade := query.Get("is_upgrade"); isUpgrade == "true" {
		req.IsUpgrade = true
	}

	if validate := query.Get("validate"); validate == "true" {
		req.Validate = true
	}

	// Array parameters
	if setValues := query["set"]; len(setValues) > 0 {
		req.StringValues = setValues
	}

	if valueFiles := query["value_files"]; len(valueFiles) > 0 {
		req.ValueFiles = valueFiles
	}

	if fileValues := query["file_values"]; len(fileValues) > 0 {
		req.FileValues = fileValues
	}

	if jsonValues := query["json_values"]; len(jsonValues) > 0 {
		req.JSONValues = jsonValues
	}

	if showOnly := query["show_only"]; len(showOnly) > 0 {
		req.ShowOnly = showOnly
	}

	if apiVersions := query["api_versions"]; len(apiVersions) > 0 {
		req.APIVersions = apiVersions
	}

	return req, nil
}
