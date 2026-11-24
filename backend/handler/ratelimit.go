package handler

import (
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// limiterStore keeps one token-bucket limiter per key (a client IP or a
// username). Limits are per server instance: with 2 App Engine instances a
// client can get up to twice the limit.
type limiterStore struct {
	mu       sync.Mutex
	limit    rate.Limit
	burst    int
	visitors map[string]*visitor
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newLimiterStore(limit rate.Limit, burst int) *limiterStore {
	store := &limiterStore{limit: limit, burst: burst, visitors: map[string]*visitor{}}
	go store.removeIdleVisitors()
	return store
}

func (s *limiterStore) get(key string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.visitors[key]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(s.limit, s.burst)}
		s.visitors[key] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// removeIdleVisitors frees limiters of clients that have been quiet for 10
// minutes, so the map does not grow forever.
func (s *limiterStore) removeIdleVisitors() {
	for {
		time.Sleep(time.Minute)
		s.mu.Lock()
		for key, v := range s.visitors {
			if time.Since(v.lastSeen) > 10*time.Minute {
				delete(s.visitors, key)
			}
		}
		s.mu.Unlock()
	}
}

var (
	// Every request: 10 per second per client IP, bursts of 20.
	globalLimiter = newLimiterStore(rate.Limit(10), 20)
	// Sign up, sign in and refresh: about 10 per minute per client IP, against password guessing.
	authLimiter = newLimiterStore(rate.Every(6*time.Second), 10)
	// Uploads: about 10 per minute per user, against spam.
	uploadLimiter = newLimiterStore(rate.Every(6*time.Second), 5)
)

// behindAppEngine is true on App Engine, where requests arrive through
// Google's load balancer and the client IP is in X-Forwarded-For.
var behindAppEngine = os.Getenv("GAE_INSTANCE") != ""

// clientIP returns the IP address the request came from.
func clientIP(r *http.Request) string {
	if behindAppEngine {
		// The load balancer appends "<client ip>, <load balancer ip>". Entries
		// before those two come from the client and could be forged.
		parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		if len(parts) >= 2 {
			return strings.TrimSpace(parts[len(parts)-2])
		}
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func byIP(r *http.Request) string {
	return "ip:" + clientIP(r)
}

// byUser limits signed-in users by username, so one user cannot get around
// the limit by switching networks. It needs the JWT middleware to run first.
func byUser(r *http.Request) string {
	if username := usernameFromRequest(r); username != "" {
		return "user:" + username
	}
	return byIP(r)
}

// rateLimit answers 429 Too Many Requests, with a Retry-After header, once a
// key has used up its limit.
func rateLimit(store *limiterStore, keyOf func(*http.Request) string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		limiter := store.get(keyOf(r))
		if !limiter.Allow() {
			reservation := limiter.Reserve()
			wait := reservation.Delay()
			reservation.Cancel()
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			http.Error(w, "Too many requests, please slow down", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
