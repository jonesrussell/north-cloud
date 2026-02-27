package proxypool

import (
	"net/http"
	"net/url"
)

// ProxyFunc returns a function compatible with http.Transport.Proxy and Colly's
// SetProxyFunc. It extracts the hostname from the request URL and calls ProxyFor.
func (p *Pool) ProxyFunc() func(*http.Request) (*url.URL, error) {
	return func(r *http.Request) (*url.URL, error) {
		return p.ProxyFor(r.URL.Hostname())
	}
}

// NewTransport creates an *http.Transport with its Proxy set to the pool's ProxyFunc.
// If base is non-nil, it is cloned and the Proxy field is overridden. If base is nil,
// a default transport is created.
func NewTransport(pool *Pool, base *http.Transport) *http.Transport {
	var t *http.Transport

	if base != nil {
		t = base.Clone()
	} else {
		t = &http.Transport{}
	}

	t.Proxy = pool.ProxyFunc()

	return t
}
