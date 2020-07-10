package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/julienschmidt/httprouter"

	"github.com/beatlabs/patron/component/http/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockAuthenticator struct {
	success bool
	err     error
}

func (mo MockAuthenticator) Authenticate(req *http.Request) (bool, error) {
	if mo.err != nil {
		return false, mo.err
	}
	return mo.success, nil
}

func TestRouteBuilder_WithMethodGet(t *testing.T) {
	type args struct {
		methodExists bool
	}
	tests := map[string]struct {
		args        args
		expectedErr string
	}{
		"success":               {args: args{}},
		"method already exists": {args: args{methodExists: true}, expectedErr: "method already set\n"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rb := NewRawRouteBuilder("/", func(http.ResponseWriter, *http.Request) {})
			if tt.args.methodExists {
				rb.MethodGet()
			}
			got, err := rb.MethodGet().Build()

			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				assert.Equal(t, Route{}, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, 1, len(got))
				assert.Equal(t, http.MethodGet, got[0].method)
			}
		})
	}
}

func TestRouteBuilder_WithMethodPost(t *testing.T) {
	rb := NewRawRouteBuilder("/", func(http.ResponseWriter, *http.Request) {}).MethodPost()
	assert.Equal(t, http.MethodPost, rb.method)
}

func TestRouteBuilder_WithMethodPut(t *testing.T) {
	rb := NewRawRouteBuilder("/", func(http.ResponseWriter, *http.Request) {}).MethodPut()
	assert.Equal(t, http.MethodPut, rb.method)
}

func TestRouteBuilder_WithMethodPatch(t *testing.T) {
	rb := NewRawRouteBuilder("/", func(http.ResponseWriter, *http.Request) {}).MethodPatch()
	assert.Equal(t, http.MethodPatch, rb.method)
}

func TestRouteBuilder_WithMethodConnect(t *testing.T) {
	rb := NewRawRouteBuilder("/", func(http.ResponseWriter, *http.Request) {}).MethodConnect()
	assert.Equal(t, http.MethodConnect, rb.method)
}

func TestRouteBuilder_WithMethodDelete(t *testing.T) {
	rb := NewRawRouteBuilder("/", func(http.ResponseWriter, *http.Request) {}).MethodDelete()
	assert.Equal(t, http.MethodDelete, rb.method)
}

func TestRouteBuilder_WithMethodHead(t *testing.T) {
	rb := NewRawRouteBuilder("/", func(http.ResponseWriter, *http.Request) {}).MethodHead()
	assert.Equal(t, http.MethodHead, rb.method)
}

func TestRouteBuilder_WithMethodTrace(t *testing.T) {
	rb := NewRawRouteBuilder("/", func(http.ResponseWriter, *http.Request) {}).MethodTrace()
	assert.Equal(t, http.MethodTrace, rb.method)
}

func TestRouteBuilder_WithMethodOptions(t *testing.T) {
	rb := NewRawRouteBuilder("/", func(http.ResponseWriter, *http.Request) {}).MethodOptions()
	assert.Equal(t, http.MethodOptions, rb.method)
}

func TestRouteBuilder_WithTrace(t *testing.T) {
	rb := NewRawRouteBuilder("/", func(http.ResponseWriter, *http.Request) {}).WithTrace()
	assert.True(t, rb.trace)
}

func TestRouteBuilder_WithMiddlewares(t *testing.T) {
	middleware := func(next http.Handler) http.Handler { return next }
	mockHandler := func(http.ResponseWriter, *http.Request) {}
	type fields struct {
		middlewares []MiddlewareFunc
	}
	tests := map[string]struct {
		fields      fields
		expectedErr string
	}{
		"success":            {fields: fields{middlewares: []MiddlewareFunc{middleware}}},
		"missing middleware": {fields: fields{}, expectedErr: "middlewares are empty"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rb := NewRawRouteBuilder("/", mockHandler).MethodGet()
			if len(tt.fields.middlewares) == 0 {
				rb.WithMiddlewares()
			} else {
				rb.WithMiddlewares(tt.fields.middlewares...)
			}

			if tt.expectedErr != "" {
				assert.Len(t, rb.errors, 1)
				assert.EqualError(t, rb.errors[0], tt.expectedErr)
			} else {
				assert.Len(t, rb.errors, 0)
				assert.Len(t, rb.middlewares, 1)
			}
		})
	}
}

func TestRouteBuilder_WithAuth(t *testing.T) {
	mockAuth := &MockAuthenticator{}
	mockHandler := func(http.ResponseWriter, *http.Request) {}
	type fields struct {
		authenticator auth.Authenticator
	}
	tests := map[string]struct {
		fields      fields
		expectedErr string
	}{
		"success":            {fields: fields{authenticator: mockAuth}},
		"missing middleware": {fields: fields{}, expectedErr: "authenticator is nil"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rb := NewRawRouteBuilder("/", mockHandler).WithAuth(tt.fields.authenticator)

			if tt.expectedErr != "" {
				assert.Len(t, rb.errors, 1)
				assert.EqualError(t, rb.errors[0], tt.expectedErr)
			} else {
				assert.Len(t, rb.errors, 0)
				assert.NotNil(t, rb.authenticator)
			}
		})
	}
}

func TestRouteBuilder_Build(t *testing.T) {
	mockAuth := &MockAuthenticator{}
	mockProcessor := func(context.Context, *Request) (*Response, error) { return nil, nil }
	middleware := func(next http.Handler) http.Handler { return next }
	type fields struct {
		path          string
		missingMethod bool
	}
	tests := map[string]struct {
		fields      fields
		expectedErr string
	}{
		"success":           {fields: fields{path: "/"}},
		"missing processor": {fields: fields{path: ""}, expectedErr: "path is empty\n"},
		"missing method":    {fields: fields{path: "/", missingMethod: true}, expectedErr: "method is missing"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rb := NewRouteBuilder(tt.fields.path, mockProcessor).WithTrace().WithAuth(mockAuth).WithMiddlewares(middleware)
			if !tt.fields.missingMethod {
				rb.MethodGet()
			}
			got, err := rb.Build()

			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				assert.Equal(t, Route{}, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

func TestNewRawRouteBuilder(t *testing.T) {
	mockHandler := func(http.ResponseWriter, *http.Request) {}
	type args struct {
		path    string
		handler http.HandlerFunc
	}
	tests := map[string]struct {
		args        args
		expectedErr string
	}{
		"success":         {args: args{path: "/", handler: mockHandler}},
		"invalid path":    {args: args{path: "", handler: mockHandler}, expectedErr: "path is empty"},
		"invalid handler": {args: args{path: "/", handler: nil}, expectedErr: "handler is nil"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rb := NewRawRouteBuilder(tt.args.path, tt.args.handler)

			if tt.expectedErr != "" {
				assert.Len(t, rb.errors, 1)
				assert.EqualError(t, rb.errors[0], tt.expectedErr)
			} else {
				assert.Len(t, rb.errors, 0)
			}
		})
	}
}

func TestNewRouteBuilder(t *testing.T) {
	mockProcessor := func(context.Context, *Request) (*Response, error) { return nil, nil }
	type args struct {
		path      string
		processor ProcessorFunc
	}
	tests := map[string]struct {
		args        args
		expectedErr string
	}{
		"success":         {args: args{path: "/", processor: mockProcessor}},
		"invalid path":    {args: args{path: "", processor: mockProcessor}, expectedErr: "path is empty"},
		"invalid handler": {args: args{path: "/", processor: nil}, expectedErr: "processor is nil"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			rb := NewRouteBuilder(tt.args.path, tt.args.processor)

			if tt.expectedErr != "" {
				assert.Len(t, rb.errors, 1)
				assert.EqualError(t, rb.errors[0], tt.expectedErr)
			} else {
				assert.Len(t, rb.errors, 0)
			}
		})
	}
}

func TestRoutesBuilder_Build(t *testing.T) {
	mockHandler := func(http.ResponseWriter, *http.Request) {}
	validRb := NewRawRouteBuilder("/", mockHandler).MethodGet()
	invalidRb := NewRawRouteBuilder("/", mockHandler)
	type args struct {
		rbs []*RouteBuilder
	}
	tests := map[string]struct {
		args        args
		expectedErr string
	}{
		"success": {
			args: args{rbs: []*RouteBuilder{validRb}},
		},
		"invalid route builder": {
			args:        args{rbs: []*RouteBuilder{invalidRb}},
			expectedErr: "method is missing\n",
		},
		"duplicate routes": {
			args:        args{rbs: []*RouteBuilder{validRb, validRb}},
			expectedErr: "route with key get-/ is duplicate\n",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			builder := NewRoutesBuilder()
			for _, rb := range tt.args.rbs {
				builder.Append(rb)
			}
			got, err := builder.Build()

			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, 1)
			}
		})
	}
}

func TestRoute_Getters(t *testing.T) {
	type testResponse struct {
		Value string
	}

	path := "/foo"
	expectedResponse := testResponse{"foo"}
	rr, err := NewRouteBuilder(path, testingHandlerMock(expectedResponse)).WithTrace().MethodPost().Build()
	require.NoError(t, err)

	assert.Len(t, rr, 1)
	r := rr[0]
	assert.Equal(t, path, r.Path())
	assert.Equal(t, http.MethodPost, r.Method())
	assert.Len(t, r.Middlewares(), 1)

	// the only way to test do we get the same handler that we provided initially, is to run it explicitly,
	// since all we have in Route itself is a wrapper function
	req, err := http.NewRequest(http.MethodPost, path, nil)
	require.NoError(t, err)
	wr := httptest.NewRecorder()

	r.Handler().ServeHTTP(wr, req)
	br, err := ioutil.ReadAll(wr.Body)
	require.NoError(t, err)

	gotResponse := testResponse{}
	err = json.Unmarshal(br, &gotResponse)
	require.NoError(t, err)

	assert.Equal(t, expectedResponse, gotResponse)
}

func Test_ApplicationTypeVersionSubtypeRegex(t *testing.T) {
	regex := regexp.MustCompile(applicationTypeVersionSubtypeRegex)
	tests := []struct {
		acceptHeader string
		succeeds     bool
		subtype      string
		version      string
	}{
		{"", false, "", ""},
		{"application/json", false, "", ""},
		{"application/vnd.patron.v2+json", true, "vnd.patron", "2"},
		{"application/vnd.patron.name.v2.1+json", true, "vnd.patron.name", "2.1"},
		{"application/vnd.patron.name.v12.31+json", true, "vnd.patron.name", "12.31"},
		{"application/vnd.patron.name.v2.1+xml;charset=UTF-8", true, "vnd.patron.name", "2.1"},
		{"application/vnd.patron.name.v1+json; charset=UTF-8;x=y", true, "vnd.patron.name", "1"},
		{"application/vnd..patron.name.v1+json; charset=UTF-8;x=y", false, "", ""},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("accept header %s", tt.acceptHeader), func(t *testing.T) {
			match := regex.FindStringSubmatch(tt.acceptHeader)
			if tt.succeeds {
				assert.Equal(t, 3, len(match))
				assert.Equal(t, tt.subtype, match[1])
				assert.Equal(t, tt.version, match[2])
			} else {
				assert.Nil(t, match)
			}
		})
	}
}

func Test_ApplicationTypeVersionParameterRegex(t *testing.T) {
	regex := regexp.MustCompile(applicationTypeVersionParameterRegex)
	tests := []struct {
		acceptHeader string
		succeeds     bool
		subtype      string
		version      string
	}{
		{"", false, "", ""},
		{"application/json", false, "", ""},
		{"application/vnd.patron+json; version=2", true, "vnd.patron", "2"},
		{"application/vnd.patron.name+json;version=2.1", true, "vnd.patron.name", "2.1"},
		{"application/vnd.patron.name+json; version=12.31", true, "vnd.patron.name", "12.31"},
		{"application/vnd.patron.name+json;version=1.2; charset=UTF-8", true, "vnd.patron.name", "1.2"},
		{"application/vnd.patron.name+json; something=value;version=1.2;charset=UTF-8", true, "vnd.patron.name", "1.2"},
		{"application/vnd.patron.name+json; something=value; version=1.2; charset=UTF-8", true, "vnd.patron.name", "1.2"},
		{"application/vnd.patron..name+json; something=value; version=1;charset=UTF-8", false, "", ""},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("accept header %s", tt.acceptHeader), func(t *testing.T) {
			match := regex.FindStringSubmatch(tt.acceptHeader)
			if tt.succeeds {
				assert.Equal(t, 3, len(match))
				assert.Equal(t, tt.subtype, match[1])
				assert.Equal(t, tt.version, match[2])
			} else {
				assert.Nil(t, match)
			}
		})
	}
}

func TestBla(t *testing.T) {

	handlerv1 := func(context.Context, *Request) (*Response, error) {
		println("Inside handler v1")
		return &Response{1}, nil
	}
	handlerv2 := func(context.Context, *Request) (*Response, error) {
		println("Inside handler v2")
		return &Response{2}, nil
	}
	routes, err := NewRoutesBuilder().
		Append(NewRouteBuilder("/bar", handlerv1).MethodGet()).
		Append(
			NewVersionedRouteBuilder("/foo", map[int]ProcessorFunc{1: handlerv1, 2: handlerv2}).
				WithDefaultVersion(1).
				MethodGet()).
		// -> /foo
		// -> /foo header=application/vnd.something+json;version=1
		// -> /foo header=application/vnd.something+json;version=2
		// -> /v1/foo
		// -> /v2/foo
		Build()

	assert.NoError(t, err)
	assert.Equal(t, 4, len(routes))

	router := httprouter.New()
	for _, r := range routes {
		router.Handler(r.method, r.path, r.handler)

	}
	routerAfterMiddleware := MiddlewareChain(router, NewRecoveryMiddleware())

	s := &http.Server{
		Addr:    ":3000",
		Handler: routerAfterMiddleware,
	}

	err = s.ListenAndServe()

}

func testingHandlerMock(expected interface{}) ProcessorFunc {
	return func(_ context.Context, _ *Request) (*Response, error) {
		return NewResponse(expected), nil
	}
}
