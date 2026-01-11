package proxy

import (
	"hash/fnv"
	"net/http"
	"sync"
	"sync/atomic"
)

// LoadBalancer handles backend selection
type LoadBalancer struct {
	backends []*Backend
	strategy LoadBalanceStrategy
	current  atomic.Int32
	mu       sync.RWMutex
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(strategy LoadBalanceStrategy, backends []*Backend) *LoadBalancer {
	// Initialize health for all backends
	for _, backend := range backends {
		if backend.Health == nil {
			backend.Health = &Health{
				Status: HealthStatusHealthy,
			}
		}
	}

	lb := &LoadBalancer{
		backends: backends,
		strategy: strategy,
	}

	return lb
}

// AddBackend adds a backend to the load balancer
func (lb *LoadBalancer) AddBackend(backend *Backend) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if backend.Health == nil {
		backend.Health = &Health{
			Status: HealthStatusHealthy,
		}
	}

	lb.backends = append(lb.backends, backend)
}

// RemoveBackend removes a backend by name
func (lb *LoadBalancer) RemoveBackend(name string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	for i, backend := range lb.backends {
		if backend.Name == name {
			lb.backends = append(lb.backends[:i], lb.backends[i+1:]...)
			return
		}
	}
}

// GetBackends returns all backends
func (lb *LoadBalancer) GetBackends() []*Backend {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	backends := make([]*Backend, len(lb.backends))
	copy(backends, lb.backends)
	return backends
}

// SelectBackend selects a backend using the configured strategy
func (lb *LoadBalancer) SelectBackend(req *http.Request) *Backend {
	// Get healthy backends
	healthy := lb.healthyBackends()
	if len(healthy) == 0 {
		return nil
	}

	// Select based on strategy
	switch lb.strategy {
	case StrategyRoundRobin:
		return lb.roundRobin(healthy)
	case StrategyLeastConn:
		return lb.leastConn(healthy)
	case StrategyIPHash:
		return lb.ipHash(req, healthy)
	case StrategyWeighted:
		return lb.weighted(healthy)
	default:
		return lb.roundRobin(healthy)
	}
}

// roundRobin selects backends in round-robin order
func (lb *LoadBalancer) roundRobin(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	idx := lb.current.Add(1)
	return backends[int(idx)%len(backends)]
}

// leastConn selects the backend with fewest connections
func (lb *LoadBalancer) leastConn(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	var selected *Backend
	minConn := int32(1<<31 - 1)

	for _, backend := range backends {
		conn := backend.Health.GetConnections()
		if conn < minConn {
			minConn = conn
			selected = backend
		}
	}

	if selected == nil {
		return backends[0]
	}

	return selected
}

// ipHash selects a backend based on client IP hash
func (lb *LoadBalancer) ipHash(req *http.Request, backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	// Extract client IP
	clientIP, _, _ := extractClientIP(req)

	// Hash IP to backend index
	hash := fnv.New32a()
	hash.Write([]byte(clientIP))
	idx := hash.Sum32() % uint32(len(backends))

	return backends[idx]
}

// weighted selects a backend using weighted random selection
func (lb *LoadBalancer) weighted(backends []*Backend) *Backend {
	if len(backends) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0
	for _, b := range backends {
		weight := b.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
	}

	if totalWeight == 0 {
		return backends[0]
	}

	// Use round-robin counter for deterministic selection
	idx := int(lb.current.Add(1)) % totalWeight

	// Find backend by weight
	for _, b := range backends {
		weight := b.Weight
		if weight <= 0 {
			weight = 1
		}
		idx -= weight
		if idx < 0 {
			return b
		}
	}

	return backends[0]
}

// healthyBackends returns only healthy backends
func (lb *LoadBalancer) healthyBackends() []*Backend {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	healthy := make([]*Backend, 0, len(lb.backends))
	for _, backend := range lb.backends {
		if backend.Health.GetStatus() == HealthStatusHealthy {
			healthy = append(healthy, backend)
		}
	}

	return healthy
}
