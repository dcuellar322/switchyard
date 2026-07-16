package adapters

import (
	"context"
	"net/http"
	"net/http/httputil"

	"switchyard.dev/switchyard/internal/routing/domain"
)

// Resolver returns the current bounded local-route resolution.
type Resolver interface {
	Resolve(context.Context, string) (domain.Route, error)
}

// Proxy is an HTTP-only reverse proxy over a validated routing registry.
type Proxy struct{ routes Resolver }

// NewProxy creates a local route handler. Listener binding remains a bootstrap
// concern and must be limited to loopback by the composition root.
func NewProxy(routes Resolver) *Proxy { return &Proxy{routes: routes} }

// ServeHTTP forwards only active .localhost routes to validated loopback HTTP
// targets. It does not terminate TLS or dynamically resolve remote upstreams.
func (p *Proxy) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	route, err := p.routes.Resolve(request.Context(), request.Host)
	if err != nil {
		writeProxyStatus(response, http.StatusBadRequest, "invalid local route host")
		return
	}
	switch route.Status {
	case domain.StatusActive:
		p.forward(response, request, route)
	case domain.StatusConflict:
		writeProxyStatus(response, http.StatusConflict, route.Reason)
	case domain.StatusDisabled, domain.StatusUnavailable:
		writeProxyStatus(response, http.StatusServiceUnavailable, route.Reason)
	default:
		writeProxyStatus(response, http.StatusServiceUnavailable, "local route is unavailable")
	}
}

func (p *Proxy) forward(response http.ResponseWriter, request *http.Request, route domain.Route) {
	target, err := domain.ValidateTarget(route.Target)
	if err != nil {
		writeProxyStatus(response, http.StatusServiceUnavailable, "local route target is unavailable")
		return
	}
	originalHost := request.Host
	request = request.Clone(request.Context())
	request.Header = request.Header.Clone()
	proxy := &httputil.ReverseProxy{
		Rewrite: func(proxyRequest *httputil.ProxyRequest) {
			proxyRequest.SetURL(target)
			proxyRequest.SetXForwarded()
			proxyRequest.Out.Host = target.Host
			proxyRequest.Out.Header.Set("X-Forwarded-Host", originalHost)
			proxyRequest.Out.Header.Set("X-Forwarded-Proto", "http")
		},
		ErrorHandler: func(writer http.ResponseWriter, _ *http.Request, _ error) {
			writeProxyStatus(writer, http.StatusBadGateway, "local route target did not respond")
		},
	}
	removeForwardingHeaders(request.Header)
	proxy.ServeHTTP(response, request)
}

func removeForwardingHeaders(header http.Header) {
	for _, name := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto"} {
		header.Del(name)
	}
}

func writeProxyStatus(response http.ResponseWriter, status int, message string) {
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_, _ = response.Write([]byte(message + "\n"))
}
