package template

import (
	"net/http"
	"testing"

	"github.com/appkins-org/helm-api/internal/config"
	"github.com/appkins-org/helm-api/internal/template"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/gorilla/schema"
)

// TestParseTemplateRequest tests the query parameter parsing functionality.
func TestParseTemplateRequest(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError bool
		expected    func(req template.TemplateRequest) bool
	}{
		{
			name:        "missing chart parameter",
			url:         "/template",
			expectError: true,
		},
		{
			name:        "basic chart parameter",
			url:         "/template?chart=test-chart",
			expectError: false,
			expected: func(req template.TemplateRequest) bool {
				return req.Chart == "test-chart"
			},
		},
		{
			name:        "full parameter set",
			url:         "/template?chart=test-chart&release_name=test-release&namespace=test-ns&kube_version=v1.29.0&include_crds=true&skip_tests=true&is_upgrade=false&validate=true",
			expectError: false,
			expected: func(req template.TemplateRequest) bool {
				return req.Chart == "test-chart" &&
					req.ReleaseName == "test-release" &&
					req.Namespace == "test-ns" &&
					req.KubeVersion == "v1.29.0" &&
					req.IncludeCRDs == true &&
					req.SkipTests == true &&
					req.IsUpgrade == false &&
					req.Validate == true
			},
		},
		{
			name:        "multiple set values",
			url:         "/template?chart=test-chart&set=key1=value1&set=key2=value2&set=nested.key=value3",
			expectError: false,
			expected: func(req template.TemplateRequest) bool {
				return req.Chart == "test-chart" &&
					len(req.Values) == 3 &&
					req.Values[0] == "key1=value1" &&
					req.Values[1] == "key2=value2" &&
					req.Values[2] == "nested.key=value3"
			},
		},
		{
			name:        "multiple array parameters",
			url:         "/template?chart=test-chart&show_only=deployment.yaml&show_only=service.yaml&api_versions=apps/v1&api_versions=v1",
			expectError: false,
			expected: func(req template.TemplateRequest) bool {
				return req.Chart == "test-chart" &&
					len(req.ShowOnly) == 2 &&
					req.ShowOnly[0] == "deployment.yaml" &&
					req.ShowOnly[1] == "service.yaml" &&
					len(req.APIVersions) == 2 &&
					req.APIVersions[0] == "apps/v1" &&
					req.APIVersions[1] == "v1"
			},
		},
		{
			name:        "chart with repository and version",
			url:         "/template?chart=nginx&repository=https://charts.bitnami.com/bitnami&chart_version=15.0.0",
			expectError: false,
			expected: func(req template.TemplateRequest) bool {
				return req.Chart == "nginx" &&
					req.Repository == "https://charts.bitnami.com/bitnami" &&
					req.ChartVersion == "15.0.0"
			},
		},
		{
			name:        "OCI chart",
			url:         "/template?chart=oci://registry-1.docker.io/bitnamicharts/nginx&chart_version=15.0.0",
			expectError: false,
			expected: func(req template.TemplateRequest) bool {
				return req.Chart == "oci://registry-1.docker.io/bitnamicharts/nginx" &&
					req.ChartVersion == "15.0.0"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock HTTP request with the test URL
			httpReq, err := http.NewRequest("GET", tt.url, nil)
			if err != nil {
				t.Fatal(err)
			}

			h := handler{
				logger:  logr.FromContextAsSlogLogger(logr.NewContext(t.Context(), testr.New(t))),
				config:  config.DefaultConfig(),
				decoder: schema.NewDecoder(),
			}

			req, err := h.parseTemplateRequest(httpReq)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.expected != nil && !tt.expected(req) {
					t.Errorf("Request did not match expected values for %s", tt.name)
				}
			}
		})
	}
}
