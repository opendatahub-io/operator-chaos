package chaostransport

import (
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
)

// ChaosTransport wraps an http.RoundTripper with fault injection at the HTTP level.
// Unlike ChaosClient (which only wraps client.Client and misses cache reads),
// ChaosTransport intercepts ALL HTTP requests including informer watches,
// cache list/gets, and direct API calls.
//
// Use with rest.Config.WrapTransport:
//
//	cfg := ctrl.GetConfigOrDie()
//	ct := sdk.NewChaosTransport(sdk.NewFaultConfig(nil))
//	cfg.WrapTransport = ct.WrapTransport
type ChaosTransport struct {
	faults atomic.Pointer[FaultConfig]
}

// NewChaosTransport creates a new ChaosTransport with the given fault config.
func NewChaosTransport(fc *FaultConfig) *ChaosTransport {
	ct := &ChaosTransport{}
	if fc != nil {
		ct.faults.Store(fc)
	}
	return ct
}

// UpdateFaultConfig atomically replaces the fault configuration.
func (ct *ChaosTransport) UpdateFaultConfig(fc *FaultConfig) {
	ct.faults.Store(fc)
}

// WrapTransport returns a function suitable for rest.Config.WrapTransport.
func (ct *ChaosTransport) WrapTransport(rt http.RoundTripper) http.RoundTripper {
	return &chaosRoundTripper{
		inner:     rt,
		transport: ct,
	}
}

type chaosRoundTripper struct {
	inner     http.RoundTripper
	transport *ChaosTransport
}

func (rt *chaosRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	fc := rt.transport.faults.Load()
	if fc == nil || !fc.IsActive() {
		return rt.inner.RoundTrip(req)
	}

	// Exclude chaos ConfigMap reads (GET, LIST, WATCH) from fault injection
	// so the config watcher can always read its own configuration.
	if strings.Contains(req.URL.Path, "/configmaps") &&
		strings.Contains(req.URL.Path, ChaosConfigMapName) {
		return rt.inner.RoundTrip(req)
	}

	op := httpRequestToOperation(req)
	if err := fc.MaybeInject(op); err != nil {
		return &http.Response{
			StatusCode: mapChaosErrorToHTTPStatus(op),
			Status:     fmt.Sprintf("%d Chaos Injected", mapChaosErrorToHTTPStatus(op)),
			Body:       http.NoBody,
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}

	return rt.inner.RoundTrip(req)
}

func httpMethodToOperation(method string) Operation {
	switch strings.ToUpper(method) {
	case "GET":
		return OpGet
	case "PUT":
		return OpUpdate
	case "POST":
		return OpCreate
	case "PATCH":
		return OpPatch
	case "DELETE":
		return OpDelete
	default:
		return OpGet
	}
}

// httpRequestToOperation determines the operation from the full HTTP request.
// Watch requests (?watch=true) map to OpList. All other GETs map to OpGet.
// To inject faults on all read operations, configure both OpGet and OpList.
func httpRequestToOperation(req *http.Request) Operation {
	op := httpMethodToOperation(req.Method)
	if op == OpGet && req.URL.Query().Get("watch") == "true" {
		return OpList
	}
	return op
}

func mapChaosErrorToHTTPStatus(op Operation) int {
	switch op {
	case OpGet, OpList:
		return http.StatusServiceUnavailable
	case OpCreate:
		return http.StatusTooManyRequests
	case OpUpdate, OpPatch:
		return http.StatusConflict
	case OpDelete:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
