package http

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/beatlabs/patron/component/http/auth"
	errs "github.com/beatlabs/patron/errors"
)

// Route definition of a HTTP route.
type Route struct {
	path        string
	method      string
	handler     http.HandlerFunc
	middlewares []MiddlewareFunc
}

// Path returns route path value.
func (r Route) Path() string {
	return r.path
}

// Method returns route method value (GET/POST/...).
func (r Route) Method() string {
	return r.method
}

// Middlewares returns route middlewares.
func (r Route) Middlewares() []MiddlewareFunc {
	return r.middlewares
}

// Handler returns route handler function.
func (r Route) Handler() http.HandlerFunc {
	return r.handler
}

const defaultHandler = 0

// RouteBuilder for building a route.
type RouteBuilder struct {
	method        string
	path          string
	trace         bool
	middlewares   []MiddlewareFunc
	authenticator auth.Authenticator
	handlers      map[int]http.HandlerFunc
	errors        []error
}

// WithTrace enables route tracing.
func (rb *RouteBuilder) WithTrace() *RouteBuilder {
	rb.trace = true
	return rb
}

// WithMiddlewares adds middlewares.
func (rb *RouteBuilder) WithMiddlewares(mm ...MiddlewareFunc) *RouteBuilder {
	if len(mm) == 0 {
		rb.errors = append(rb.errors, errors.New("middlewares are empty"))
	}
	rb.middlewares = mm
	return rb
}

// WithAuth adds authenticator.
func (rb *RouteBuilder) WithAuth(auth auth.Authenticator) *RouteBuilder {
	if auth == nil {
		rb.errors = append(rb.errors, errors.New("authenticator is nil"))
	}
	rb.authenticator = auth
	return rb
}

func (rb *RouteBuilder) setMethod(method string) *RouteBuilder {
	if rb.method != "" {
		rb.errors = append(rb.errors, errors.New("method already set"))
	}
	rb.method = method
	return rb
}

// MethodGet HTTP method.
func (rb *RouteBuilder) MethodGet() *RouteBuilder {
	return rb.setMethod(http.MethodGet)
}

// MethodHead HTTP method.
func (rb *RouteBuilder) MethodHead() *RouteBuilder {
	return rb.setMethod(http.MethodHead)
}

// MethodPost HTTP method.
func (rb *RouteBuilder) MethodPost() *RouteBuilder {
	return rb.setMethod(http.MethodPost)
}

// MethodPut HTTP method.
func (rb *RouteBuilder) MethodPut() *RouteBuilder {
	return rb.setMethod(http.MethodPut)
}

// MethodPatch HTTP method.
func (rb *RouteBuilder) MethodPatch() *RouteBuilder {
	return rb.setMethod(http.MethodPatch)
}

// MethodDelete HTTP method.
func (rb *RouteBuilder) MethodDelete() *RouteBuilder {
	return rb.setMethod(http.MethodDelete)
}

// MethodConnect HTTP method.
func (rb *RouteBuilder) MethodConnect() *RouteBuilder {
	return rb.setMethod(http.MethodConnect)
}

// MethodOptions HTTP method.
func (rb *RouteBuilder) MethodOptions() *RouteBuilder {
	return rb.setMethod(http.MethodOptions)
}

// MethodTrace HTTP method.
func (rb *RouteBuilder) MethodTrace() *RouteBuilder {
	return rb.setMethod(http.MethodTrace)
}

// Build a route.
func (rb *RouteBuilder) Build() ([]Route, error) {
	if len(rb.errors) > 0 {
		return []Route{}, errs.Aggregate(rb.errors...)
	}

	if rb.method == "" {
		return []Route{}, errors.New("method is missing")
	}

	var middlewares []MiddlewareFunc
	if rb.trace {
		middlewares = append(middlewares, NewLoggingTracingMiddleware(rb.path))
	}
	if rb.authenticator != nil {
		middlewares = append(middlewares, NewAuthMiddleware(rb.authenticator))
	}
	if len(rb.middlewares) > 0 {
		middlewares = append(middlewares, rb.middlewares...)
	}

	if dh, ok := rb.handlers[defaultHandler]; len(rb.handlers) == 1 && ok {
		return []Route{{
			path:        rb.path,
			method:      rb.method,
			handler:     dh,
			middlewares: middlewares,
		}}, nil
	} else {
		re := regexp.MustCompile(`application/vnd.([a-z0-9.]+)\+([A-Za-z]+);\s*version=(\d+)`)
		versionHandler := func(handlers map[int]http.HandlerFunc) http.HandlerFunc {
			return func(rw http.ResponseWriter, rq *http.Request) {
				acceptHeader := rq.Header.Get("Accept")
				matches := re.FindStringSubmatch(acceptHeader)
				if len(matches) == 4 {
					version, err := strconv.Atoi(matches[3])
					if err == nil {
						handler, ok := handlers[version]
						if !ok {
							rw.WriteHeader(http.StatusTeapot)
						} else {
							handler(rw, rq)
						}
					}
				} else if dh, ok := handlers[defaultHandler]; ok {
					dh(rw, rq)
				} else {
					rw.WriteHeader(http.StatusExpectationFailed)
				}
			}
		}(rb.handlers)
		routes := []Route{{
			path:        rb.path,
			method:      rb.method,
			handler:     versionHandler,
			middlewares: middlewares,
		}}
		for version, handler := range rb.handlers {
			if version != defaultHandler {
				routes = append(routes, Route{path: fmt.Sprintf("/v%d%s", version, rb.path), method: rb.method, handler: handler, middlewares: middlewares})
			}
		}
		return routes, nil
	}

}

// NewRawRouteBuilder constructor.
func NewRawRouteBuilder(path string, handler http.HandlerFunc) *RouteBuilder {
	var ee []error

	if path == "" {
		ee = append(ee, errors.New("path is empty"))
	}

	if handler == nil {
		ee = append(ee, errors.New("handler is nil"))
	}

	return &RouteBuilder{path: path, errors: ee, handlers: map[int]http.HandlerFunc{defaultHandler: handler}}
}

// NewRouteBuilder constructor.
func NewRouteBuilder(path string, processor ProcessorFunc) *RouteBuilder {

	var err error

	if processor == nil {
		err = errors.New("processor is nil")
	}

	rb := NewRawRouteBuilder(path, handler(processor))
	if err != nil {
		rb.errors = append(rb.errors, err)
	}
	return rb
}

// NewVersionedRouteBuilder constructor.
func NewVersionedRouteBuilder(path string, processors map[int]ProcessorFunc) *RouteBuilder {
	var ee []error

	if path == "" {
		ee = append(ee, errors.New("path is empty"))
	}

	handlers := make(map[int]http.HandlerFunc, len(processors)+1)
	for version, processor := range processors {
		if version <= 0 {
			ee = append(ee, errors.New(fmt.Sprintf("versions smaller than 1 are not allowed")))
		} else if processor == nil {
			ee = append(ee, errors.New(fmt.Sprintf("processor for version %d is nil", version)))
		} else {
			handlers[version] = handler(processor)
		}
	}

	return &RouteBuilder{path: path, errors: ee, handlers: handlers}
}

// WithDefaultVersion set's a default version if a mediatype version is not provided.
// Using this builder step only makes sense when building a versioned route.
func (rb *RouteBuilder) WithDefaultVersion(defaultVersion int) *RouteBuilder {
	if _, ok := rb.handlers[defaultVersion]; !ok {
		rb.errors = append(rb.errors, errors.New(fmt.Sprintf("Default version %d is not present in map of versions", defaultVersion)))
	} else {
		rb.handlers[defaultHandler] = rb.handlers[defaultVersion]
	}
	return rb
}

// RoutesBuilder creates a list of routes.
type RoutesBuilder struct {
	routes []Route
	errors []error
}

// Append a route to the list.
func (rb *RoutesBuilder) Append(builder *RouteBuilder) *RoutesBuilder {
	routes, err := builder.Build()
	if err != nil {
		rb.errors = append(rb.errors, err)
	} else {
		rb.routes = append(rb.routes, routes...)
	}
	return rb
}

// Build the routes.
func (rb *RoutesBuilder) Build() ([]Route, error) {
	duplicates := make(map[string]struct{}, len(rb.routes))

	for _, r := range rb.routes {
		key := strings.ToLower(r.method + "-" + r.path)
		_, ok := duplicates[key]
		if ok {
			rb.errors = append(rb.errors, fmt.Errorf("route with key %s is duplicate", key))
			continue
		}
		duplicates[key] = struct{}{}
	}

	if len(rb.errors) > 0 {
		return nil, errs.Aggregate(rb.errors...)
	}

	return rb.routes, nil
}

// NewRoutesBuilder constructor.
func NewRoutesBuilder() *RoutesBuilder {
	return &RoutesBuilder{}
}
