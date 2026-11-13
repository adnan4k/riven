// Package manifest loads the riven.json file written by the PHP collector
// (php artisan riven:export). The contract is ../schema/riven.schema.json.
//
// Unknown values in the manifest are JSON null. They map to nil here (nil
// pointers, nil maps), so "unknown" never looks like a real zero value.
package manifest

import (
	"strings"
	"time"
)

// SupportedVersion is the schema_version this engine understands.
// Additive changes keep the version, so unknown fields are ignored on load.
const SupportedVersion = 1

type Manifest struct {
	SchemaVersion int       `json:"schema_version"`
	Generator     Generator `json:"generator"`
	GeneratedAt   time.Time `json:"generated_at"`
	App           App       `json:"app"`
	Routes        []Route   `json:"routes"`
	Warnings      []Warning `json:"warnings"`
}

type Generator struct {
	Name    string  `json:"name"`
	Version *string `json:"version"` // nil when the collector couldn't read its own version
}

type App struct {
	Name           *string `json:"name"`
	URL            *string `json:"url"`
	LaravelVersion string  `json:"laravel_version"`
	PHPVersion     string  `json:"php_version"`
}

type Route struct {
	ID                 string      `json:"id"` // "<first non-HEAD method> /<uri>", unique per route
	URI                string      `json:"uri"`
	Methods            []string    `json:"methods"`
	Name               *string     `json:"name"`
	Domain             *string     `json:"domain"`
	Action             Action      `json:"action"`
	Middleware         []string    `json:"middleware"`          // as declared: aliases, groups
	MiddlewareResolved []string    `json:"middleware_resolved"` // class names after expansion
	Parameters         []Parameter `json:"parameters"`
	Request            Request     `json:"request"`
	Response           Response    `json:"response"`
}

type ActionType string

const (
	ActionController ActionType = "controller"
	ActionInvokable  ActionType = "invokable"
	ActionClosure    ActionType = "closure"
)

type Action struct {
	Type   ActionType `json:"type"`
	Class  *string    `json:"class"`  // nil for closures
	Method *string    `json:"method"` // nil for closures
}

type Parameter struct {
	Name     string  `json:"name"`
	Required bool    `json:"required"`
	Pattern  *string `json:"pattern"` // from where(), nil if unconstrained
}

type Request struct {
	FormRequest *string `json:"form_request"`
	// Rules maps field => Laravel rule strings ("required", "max:255").
	// nil: unknown (no FormRequest, or rules() needs a live request).
	// Empty non-nil map: a FormRequest with no rules. encoding/json keeps
	// this distinction: null decodes to a nil map, {} to an empty one.
	Rules map[string][]string `json:"rules"`
}

type Response struct {
	Resource   *string `json:"resource"`   // API Resource class, nil if unknown
	Collection *bool   `json:"collection"` // nil exactly when Resource is nil
}

type WarningCode string

const (
	WarnControllerMiddlewareUnresolved WarningCode = "controller_middleware_unresolved"
	WarnRulesNeedRequest               WarningCode = "rules_need_request"
)

type Warning struct {
	RouteID string      `json:"route_id"`
	Code    WarningCode `json:"code"` // new codes are additive: handle unknown ones
	Message string      `json:"message"`
}

// RequiresAuth reports whether Laravel's auth middleware guards the route,
// from the resolved class names ("…\Authenticate:sanctum" included).
func (r Route) RequiresAuth() bool {
	for _, mw := range r.MiddlewareResolved {
		class, _, _ := strings.Cut(mw, ":")
		if strings.HasSuffix(class, `\Authenticate`) || strings.HasSuffix(class, `\AuthenticateWithBasicAuth`) {
			return true
		}
	}
	return false
}
