package lb

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"gorevision/internal/config"
)

type HealthChecker struct {
	backends []*Backend
	config   config.HealthCheckConfig
	client   *http.Client

	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

func NewHealthChecker(backends []*Backend, cfg config.HealthCheckConfig) *HealthChecker {
	return &HealthChecker{
		backends: backends,
		config:   cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (h *HealthChecker) Start() {
	go h.loop()
}

func (h *HealthChecker) Stop() {
	h.stopOnce.Do(func() {
		close(h.stop)
		<-h.done
	})
}

func (h *HealthChecker) loop() {
	defer close(h.done)

	h.checkAll()

	ticker := time.NewTicker(h.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.checkAll()
		case <-h.stop:
			return
		}
	}
}

func (h *HealthChecker) checkAll() {
	var wg sync.WaitGroup
	wg.Add(len(h.backends))

	for _, backend := range h.backends {
		go func(backend *Backend) {
			defer wg.Done()
			h.check(backend)
		}(backend)
	}

	wg.Wait()
}

func (h *HealthChecker) check(backend *Backend) {
	ctx, cancel := context.WithTimeout(context.Background(), h.config.Timeout)
	defer cancel()

	healthURL := *backend.URL
	healthURL.Path = singleJoiningSlash(backend.URL.Path, h.config.Path)
	healthURL.RawQuery = ""

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL.String(), nil)
	if err != nil {
		backend.MarkCheckResult(false, h.config.HealthyThreshold, h.config.UnhealthyThreshold)
		return
	}

	resp, err := h.client.Do(req)
	if err != nil {
		wasHealthy := backend.IsHealthy()
		backend.MarkCheckResult(false, h.config.HealthyThreshold, h.config.UnhealthyThreshold)
		if wasHealthy && !backend.IsHealthy() {
			log.Printf("backend %s marked unhealthy: %v", backend.URL, err)
		}
		return
	}
	defer resp.Body.Close()

	ok := resp.StatusCode >= 200 && resp.StatusCode < 400
	wasHealthy := backend.IsHealthy()
	backend.MarkCheckResult(ok, h.config.HealthyThreshold, h.config.UnhealthyThreshold)

	if wasHealthy && !backend.IsHealthy() {
		log.Printf("backend %s marked unhealthy: status=%d", backend.URL, resp.StatusCode)
	}
	if !wasHealthy && backend.IsHealthy() {
		log.Printf("backend %s marked healthy", backend.URL)
	}
}

func singleJoiningSlash(base string, path string) string {
	if path == "" {
		return base
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	if base == "" || strings.HasSuffix(base, "/") {
		return base + path
	}
	return base + "/" + path
}
