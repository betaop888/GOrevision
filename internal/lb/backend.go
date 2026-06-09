package lb

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"

	"gorevision/internal/config"
)

type Backend struct {
	URL   *url.URL
	Proxy *httputil.ReverseProxy

	activeConnections int64
	healthy           atomic.Bool
	successStreak     int64
	failureStreak     int64
}

func NewBackends(items []config.BackendConfig) ([]*Backend, error) {
	backends := make([]*Backend, 0, len(items))

	for i, item := range items {
		parsed, err := url.Parse(item.URL)
		if err != nil {
			return nil, fmt.Errorf("parse backend[%d]: %w", i, err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("backend[%d] must include scheme and host", i)
		}

		proxy := httputil.NewSingleHostReverseProxy(parsed)
		proxy.Transport = newProxyTransport()

		backend := &Backend{
			URL:   parsed,
			Proxy: proxy,
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error for backend %s: %v", backend.URL, err)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		}

		backend.healthy.Store(true)
		backends = append(backends, backend)
	}

	return backends, nil
}

func (b *Backend) IsHealthy() bool {
	return b.healthy.Load()
}

func (b *Backend) SetHealthy(value bool) {
	b.healthy.Store(value)
}

func (b *Backend) ActiveConnections() int64 {
	return atomic.LoadInt64(&b.activeConnections)
}

func (b *Backend) IncrementConnections() {
	atomic.AddInt64(&b.activeConnections, 1)
}

func (b *Backend) DecrementConnections() {
	atomic.AddInt64(&b.activeConnections, -1)
}

func (b *Backend) MarkCheckResult(ok bool, healthyThreshold int, unhealthyThreshold int) {
	if ok {
		atomic.AddInt64(&b.successStreak, 1)
		atomic.StoreInt64(&b.failureStreak, 0)

		if atomic.LoadInt64(&b.successStreak) >= int64(healthyThreshold) {
			b.SetHealthy(true)
		}
		return
	}

	atomic.AddInt64(&b.failureStreak, 1)
	atomic.StoreInt64(&b.successStreak, 0)

	if atomic.LoadInt64(&b.failureStreak) >= int64(unhealthyThreshold) {
		b.SetHealthy(false)
	}
}

func newProxyTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          4096,
		MaxIdleConnsPerHost:   1024,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
