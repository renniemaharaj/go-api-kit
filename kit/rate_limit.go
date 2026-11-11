package kit

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimitConfig sets a sustained request rate and a permitted burst per peer IP.
// All routes, methods, query strings, and cache outcomes share that allowance.
type RateLimitConfig struct {
	Requests int
	Interval time.Duration
	Burst    int

	// MaxClients defaults to 4096. New clients are rejected while the table is full.
	MaxClients int

	// IdleTTL defaults to five minutes and cannot be shorter than a full refill.
	IdleTTL time.Duration
}

type clientLimit struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter owns process-local token buckets. Share one instance across routes.
// Peer IPs come from RemoteAddr; forwarded headers are never trusted implicitly.
type RateLimiter struct {
	mu        sync.Mutex
	clients   map[string]*clientLimit
	config    RateLimitConfig
	limit     rate.Limit
	nextSweep time.Time
	now       func() time.Time
}

func NewRateLimiter(config RateLimitConfig) (*RateLimiter, error) {
	if config.Requests <= 0 || config.Interval <= 0 || config.Burst <= 0 {
		return nil, fmt.Errorf("kit: rate requests, interval, and burst must be positive")
	}

	if config.MaxClients < 0 || config.IdleTTL < 0 {
		return nil, fmt.Errorf("kit: rate MaxClients and IdleTTL cannot be negative")
	}

	if config.MaxClients == 0 {
		config.MaxClients = 4096
	}

	limit := rate.Limit(float64(config.Requests) / config.Interval.Seconds())
	refillSeconds := float64(config.Burst) / float64(limit)
	if config.IdleTTL == 0 {
		config.IdleTTL = max(5*time.Minute, time.Duration(math.Ceil(refillSeconds))*time.Second)
	}

	if config.IdleTTL.Seconds() < refillSeconds {
		return nil, fmt.Errorf("kit: rate IdleTTL must allow a full bucket refill")
	}

	return &RateLimiter{
		clients: make(map[string]*clientLimit),
		config:  config,
		limit:   limit,
		now:     time.Now,
	}, nil
}

// Middleware rejects excess requests with HTTP 429 and a Retry-After header.
// Install it before routing, authentication, and caches to count every request.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, err := peerIP(r.RemoteAddr)
		if err != nil {
			http.Error(w, "invalid client address", http.StatusBadRequest)
			return
		}

		allowed, retryAfter := l.allow(key)
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) allow(key string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if !now.Before(l.nextSweep) {
		for key, client := range l.clients {
			if now.Sub(client.lastSeen) >= l.config.IdleTTL {
				delete(l.clients, key)
			}
		}

		l.nextSweep = now.Add(min(l.config.IdleTTL, time.Minute))
	}

	client, ok := l.clients[key]
	if !ok {
		if len(l.clients) >= l.config.MaxClients {
			return false, 1
		}

		client = &clientLimit{
			limiter: rate.NewLimiter(l.limit, l.config.Burst),
		}
		l.clients[key] = client
	}

	client.lastSeen = now
	if client.limiter.AllowN(now, 1) {
		return true, 0
	}

	seconds := (1 - client.limiter.TokensAt(now)) / float64(l.limit)
	return false, max(1, int(math.Ceil(seconds)))
}

func peerIP(address string) (string, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}

	ip, err := netip.ParseAddr(host)
	if err != nil {
		return "", err
	}

	return ip.Unmap().WithZone("").String(), nil
}
