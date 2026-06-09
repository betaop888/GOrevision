package lb

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
)

const (
	AlgorithmRoundRobin       = "round_robin"
	AlgorithmLeastConnections = "least_connections"
	AlgorithmIPHash           = "ip_hash"
)

var ErrNoHealthyBackends = errors.New("no healthy backends available")

type Balancer struct {
	algorithm string
	backends  []*Backend
	rrIndex   uint64
}

func NewBalancer(algorithm string, backends []*Backend) (*Balancer, error) {
	algorithm = strings.ToLower(strings.TrimSpace(algorithm))
	switch algorithm {
	case AlgorithmRoundRobin, AlgorithmLeastConnections, AlgorithmIPHash:
	default:
		return nil, fmt.Errorf("unsupported algorithm %q", algorithm)
	}

	if len(backends) == 0 {
		return nil, fmt.Errorf("at least one backend is required")
	}

	return &Balancer{
		algorithm: algorithm,
		backends:  backends,
	}, nil
}

func (b *Balancer) Pick(r *http.Request) (*Backend, error) {
	switch b.algorithm {
	case AlgorithmRoundRobin:
		return b.pickRoundRobin()
	case AlgorithmLeastConnections:
		return b.pickLeastConnections()
	case AlgorithmIPHash:
		return b.pickIPHash(clientIP(r))
	default:
		return nil, ErrNoHealthyBackends
	}
}

func (b *Balancer) pickRoundRobin() (*Backend, error) {
	total := uint64(len(b.backends))
	start := atomic.AddUint64(&b.rrIndex, 1) - 1

	for i := uint64(0); i < total; i++ {
		index := (start + i) % total
		backend := b.backends[index]
		if backend.IsHealthy() {
			return backend, nil
		}
	}

	return nil, ErrNoHealthyBackends
}

func (b *Balancer) pickLeastConnections() (*Backend, error) {
	var selected *Backend
	minConnections := int64(math.MaxInt64)

	for _, backend := range b.backends {
		if !backend.IsHealthy() {
			continue
		}

		connections := backend.ActiveConnections()
		if selected == nil || connections < minConnections {
			selected = backend
			minConnections = connections
		}
	}

	if selected == nil {
		return nil, ErrNoHealthyBackends
	}

	return selected, nil
}

func (b *Balancer) pickIPHash(ip string) (*Backend, error) {
	healthy := make([]*Backend, 0, len(b.backends))
	for _, backend := range b.backends {
		if backend.IsHealthy() {
			healthy = append(healthy, backend)
		}
	}

	if len(healthy) == 0 {
		return nil, ErrNoHealthyBackends
	}

	hash := fnv.New32a()
	_, _ = hash.Write([]byte(ip))

	return healthy[int(hash.Sum32())%len(healthy)], nil
}

func clientIP(r *http.Request) string {
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}

	return r.RemoteAddr
}
