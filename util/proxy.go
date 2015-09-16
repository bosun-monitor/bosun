package util

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
)

// Creates a new http Proxy that forwards requests to the specified url.
// Differs from httputil.NewSingleHostReverseProxy only in that it properly sets the host header.
func NewSingleHostProxy(target *url.URL) *httputil.ReverseProxy {
	targetQuery := target.RawQuery
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = path.Join(target.Path, req.URL.Path)
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}
		req.Host = target.Host
	}
	return &httputil.ReverseProxy{Director: director}
}
