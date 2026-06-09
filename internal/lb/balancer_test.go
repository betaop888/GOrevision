package lb

import (
	"net/http"
	"testing"

	"gorevision/internal/config"
)

func TestRoundRobinSkipsUnhealthyBackends(t *testing.T) {
	backends, err := NewBackends([]config.BackendConfig{
		{URL: "http://localhost:9001"},
		{URL: "http://localhost:9002"},
		{URL: "http://localhost:9003"},
	})
	if err != nil {
		t.Fatal(err)
	}

	backends[1].SetHealthy(false)

	balancer, err := NewBalancer(AlgorithmRoundRobin, backends)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		backend, err := balancer.Pick(&http.Request{})
		if err != nil {
			t.Fatal(err)
		}
		if backend == backends[1] {
			t.Fatalf("picked unhealthy backend")
		}
	}
}

func TestLeastConnections(t *testing.T) {
	backends, err := NewBackends([]config.BackendConfig{
		{URL: "http://localhost:9001"},
		{URL: "http://localhost:9002"},
	})
	if err != nil {
		t.Fatal(err)
	}

	backends[0].IncrementConnections()
	backends[0].IncrementConnections()

	balancer, err := NewBalancer(AlgorithmLeastConnections, backends)
	if err != nil {
		t.Fatal(err)
	}

	backend, err := balancer.Pick(&http.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if backend != backends[1] {
		t.Fatalf("expected backend with fewer connections")
	}
}

func TestIPHashKeepsSameClientOnSameBackend(t *testing.T) {
	backends, err := NewBackends([]config.BackendConfig{
		{URL: "http://localhost:9001"},
		{URL: "http://localhost:9002"},
	})
	if err != nil {
		t.Fatal(err)
	}

	balancer, err := NewBalancer(AlgorithmIPHash, backends)
	if err != nil {
		t.Fatal(err)
	}

	req := &http.Request{
		Header:     http.Header{"X-Forwarded-For": []string{"192.168.1.10"}},
		RemoteAddr: "127.0.0.1:12345",
	}

	first, err := balancer.Pick(req)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 20; i++ {
		next, err := balancer.Pick(req)
		if err != nil {
			t.Fatal(err)
		}
		if next != first {
			t.Fatalf("ip_hash changed backend for same client")
		}
	}
}
