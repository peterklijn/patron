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
	version     int
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

// RouteBuilder for building a route.
type RouteBuilder struct {
	method         string
	path           string
	trace          bool
	middlewares    []MiddlewareFunc
	authenticator  auth.Authenticator
	handler        http.HandlerFunc
	errors         []error
	version        int
	defaultVersion bool
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
func (rb *RouteBuilder) Build() (Route, error) {
	if len(rb.errors) > 0 {
		return Route{}, errs.Aggregate(rb.errors...)
	}

	if rb.method == "" {
		return Route{}, errors.New("method is missing")
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

	return Route{
		path:        rb.path,
		method:      rb.method,
		handler:     rb.handler,
		middlewares: middlewares,
		version:     rb.version,
	}, nil
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

	return &RouteBuilder{path: path, errors: ee, handler: handler}
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
func NewVersionedRouteBuilder(path string, processor ProcessorFunc, version int, isDefaultVersion bool) *RouteBuilder {
	rb := NewRouteBuilder(path, processor)
	rb.version = version
	rb.defaultVersion = isDefaultVersion
	return rb
}

// RoutesBuilder creates a list of routes.
type RoutesBuilder struct {
	routes []Route
	errors []error
}

// Append a route to the list.
func (rb *RoutesBuilder) Append(builder *RouteBuilder) *RoutesBuilder {
	route, err := builder.Build()
	if err != nil {
		rb.errors = append(rb.errors, err)
	} else {
		rb.routes = append(rb.routes, route)
	}
	return rb
}

// Build the routes.
func (rb *RoutesBuilder) Build() ([]Route, error) {

	routes := make(map[string][]Route)
	for _, r := range rb.routes {
		key := strings.ToLower(r.method + "-" + r.path)
		routes[key] = append(routes[key], r)
	}

	re := regexp.MustCompile(`application/vnd.([a-z0-9.]+)\+([A-Za-z]+);\s*version=(\d+)`)
	for key, rs := range routes {
		if len(rs) > 1 {
			if rs[0].version <= 0 {
				rb.errors = append(rb.errors, fmt.Errorf("route with key %s is duplicate :(", key))
			} else {
				r := Route{
					path:        rs[0].path,
					method:      rs[0].method,
					middlewares: rs[0].middlewares,
					handler: func(rw http.ResponseWriter, rq *http.Request) {
						acceptHeader := rq.Header.Get("Accept")
						matches := re.FindStringSubmatch(acceptHeader)
						if len(matches) == 4 {
							i, err := strconv.Atoi(matches[3])
							if err == nil {
								switch i {
								case rs[0].version:
									rs[0].handler(rw, rq)
								case rs[1].version:
									rs[1].handler(rw, rq)
								default:
									println("Inside default fallback :(")
									rw.WriteHeader(400)
								}
							}
						} else {
							println("Inside else stattement, no match on accept header")
							rs[0].handler(rw, rq)
						}
					},
				}
				routes[key] = []Route{r}
			}

		}
	}

	rb.routes = []Route{}
	for _, r := range routes {
		rb.routes = append(rb.routes, r[0])
	}

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
