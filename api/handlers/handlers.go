package handlers

import "github.com/appkins-org/helm-api/api"

type Handlers struct {
	Template api.Handler
	Health   api.Handler
	Root     api.Handler
}
