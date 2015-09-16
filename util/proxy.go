package util

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Creates a new http Proxy that forwards requests to the specified url.
// Differs from httputil.NewSingleHostReverseProxy only in that it properly sets the host header.
func NewSingleHostProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := func(req *http.Request) {
		proxy.Director(req)
		req.Host = target.Host
	}
	return &httputil.ReverseProxy{Director: director}
}
