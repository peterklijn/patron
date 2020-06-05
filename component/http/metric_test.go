package http

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_metricRoute(t *testing.T) {
	routes, err := metricRoute().Build()
	assert.Len(t, routes, 1)
	assert.NoError(t, err)
	assert.Equal(t, http.MethodGet, routes[0].method)
	assert.Equal(t, "/metrics", routes[0].path)
	assert.NotNil(t, routes[0].handler)
}
