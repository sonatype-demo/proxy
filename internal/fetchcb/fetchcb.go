// Package fetchcb wraps a fetch.FetcherInterface with per-registry circuit
// breakers that trip only on genuine upstream unavailability.
//
// The stock circuit breaker in github.com/git-pkgs/registries counts every
// non-nil fetch error as a failure except for a fixed list: a 404, an
// oversized artifact and a locally held request. That still conflates a real
// outage with a valid client response, because any other 4xx — notably a 403
// policy block from Sonatype Firewall — is returned as an error and therefore
// trips the breaker. Because the breaker is keyed by upstream host, all
// ecosystems fronted by one host (e.g. firewall.sonatype.app for npm, PyPI and
// Maven) share a single breaker, so blocking a run of malicious packages opens
// it and the healthy packages that follow start failing with 502. This wrapper
// fixes that: a client response (404 / 4xx) is passed back to the caller but
// does NOT count against the breaker; only 5xx, rate limiting and transport
// errors do.
package fetchcb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/cenk/backoff"
	circuit "github.com/rubyist/circuitbreaker"

	"github.com/git-pkgs/registries/client"
	"github.com/git-pkgs/registries/fetch"
)

const (
	initialInterval = 30 * time.Second
	maxInterval     = 5 * time.Minute
	threshold       = 5
	maxURLTruncate  = 50
)

// Fetcher is a policy-aware circuit-breaker wrapper around a fetch.FetcherInterface.
type Fetcher struct {
	base     fetch.FetcherInterface
	mu       sync.RWMutex
	breakers map[string]*circuit.Breaker
}

// New wraps base with per-registry circuit breakers.
func New(base fetch.FetcherInterface) *Fetcher {
	return &Fetcher{base: base, breakers: make(map[string]*circuit.Breaker)}
}

func (f *Fetcher) breaker(registry string) *circuit.Breaker {
	f.mu.RLock()
	b, ok := f.breakers[registry]
	f.mu.RUnlock()
	if ok {
		return b
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if b, ok := f.breakers[registry]; ok {
		return b
	}

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = initialInterval
	bo.MaxInterval = maxInterval
	bo.Multiplier = 2.0
	// Retry forever, as the stock breaker does: the default MaxElapsedTime of
	// 15 minutes makes NextBackOff return backoff.Stop, after which the breaker
	// never half-opens again and an outage longer than that leaves it open for
	// the life of the process.
	bo.MaxElapsedTime = 0
	bo.Reset()

	b = circuit.NewBreakerWithOptions(&circuit.Options{
		BackOff:    bo,
		ShouldTrip: circuit.ThresholdTripFunc(threshold),
	})
	f.breakers[registry] = b
	return b
}

// Fetch downloads an artifact from url.
func (f *Fetcher) Fetch(ctx context.Context, url string) (*fetch.Artifact, error) {
	return f.FetchWithHeaders(ctx, url, nil)
}

// FetchWithHeaders downloads an artifact from url with additional headers.
func (f *Fetcher) FetchWithHeaders(ctx context.Context, url string, headers http.Header) (*fetch.Artifact, error) {
	registry := registryOf(url)
	b := f.breaker(registry)

	var (
		artifact *fetch.Artifact
		fetchErr error
	)
	err := b.Call(func() error {
		artifact, fetchErr = f.base.FetchWithHeaders(ctx, url, headers)
		if tripsBreaker(fetchErr) {
			return fetchErr
		}
		return nil // success or client response: do not count against the breaker
	}, 0)
	if err != nil {
		return nil, breakerError(registry, err)
	}
	return artifact, fetchErr
}

// Head checks an artifact's existence/metadata without downloading it.
func (f *Fetcher) Head(ctx context.Context, headURL string) (size int64, contentType string, err error) {
	registry := registryOf(headURL)
	b := f.breaker(registry)

	var headErr error
	err = b.Call(func() error {
		size, contentType, headErr = f.base.Head(ctx, headURL)
		if tripsBreaker(headErr) {
			return headErr
		}
		return nil
	}, 0)
	if err != nil {
		return 0, "", breakerError(registry, err)
	}
	return size, contentType, headErr
}

// GetBreakerState returns the current state of the circuit breakers, keyed the
// way the fetch URL's registry is keyed, for health checks and metrics.
func (f *Fetcher) GetBreakerState() map[string]string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	states := make(map[string]string, len(f.breakers))
	for registry, breaker := range f.breakers {
		if breaker.Tripped() {
			states[registry] = "open"
		} else {
			states[registry] = "closed"
		}
	}
	return states
}

// breakerError maps the breaker's own open-circuit error onto
// fetch.ErrUpstreamDown, so every refusal to contact a registry reports the
// same way to callers, and passes errors from the fetch itself through.
func breakerError(registry string, err error) error {
	if errors.Is(err, circuit.ErrBreakerOpen) {
		return fmt.Errorf("circuit breaker open for registry %s: %w", registry, fetch.ErrUpstreamDown)
	}
	return err
}

func registryOf(raw string) string {
	if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	if len(raw) > maxURLTruncate {
		return raw[:maxURLTruncate]
	}
	return raw
}

var upstreamStatusRe = regexp.MustCompile(`unexpected status (\d+)`)

// tripsBreaker reports whether a fetch error should count as a circuit-breaker
// failure. Client responses are valid answers from a healthy upstream and must
// NOT trip the breaker: a 404 (fetch.ErrNotFound), any 4xx such as a 403 policy
// block, an artifact over the size limit, a request held locally by rate
// limiting, or a caller-cancelled context. Genuine unavailability does: 5xx
// (fetch.ErrUpstreamDown), rate limiting (fetch.ErrRateLimited) and transport
// errors.
func tripsBreaker(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, fetch.ErrNotFound) ||
		errors.Is(err, fetch.ErrArtifactTooLarge) ||
		errors.Is(err, client.ErrUpstreamHeld) {
		return false
	}
	if errors.Is(err, fetch.ErrUpstreamDown) || errors.Is(err, fetch.ErrRateLimited) {
		return true
	}
	if m := upstreamStatusRe.FindStringSubmatch(err.Error()); m != nil {
		if code, convErr := strconv.Atoi(m[1]); convErr == nil && code >= 400 && code < 500 {
			return false
		}
	}
	return true
}
