package fetchcb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/git-pkgs/registries/client"
	"github.com/git-pkgs/registries/fetch"
)

// fakeFetcher is a fetch.FetcherInterface that always returns a fixed result and
// counts how many times it was actually called.
type fakeFetcher struct {
	mu    sync.Mutex
	calls int
	art   *fetch.Artifact
	err   error
}

func (f *fakeFetcher) count() int { f.mu.Lock(); defer f.mu.Unlock(); return f.calls }

func (f *fakeFetcher) Fetch(ctx context.Context, url string) (*fetch.Artifact, error) {
	return f.FetchWithHeaders(ctx, url, nil)
}

func (f *fakeFetcher) FetchWithHeaders(_ context.Context, _ string, _ http.Header) (*fetch.Artifact, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return f.art, f.err
}

func (f *fakeFetcher) Head(context.Context, string) (int64, string, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	return 0, "", f.err
}

func TestTripsBreaker(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil is success", nil, false},
		{"404 not-found does not trip", fetch.ErrNotFound, false},
		{"403 policy block does not trip", errors.New("unexpected status 403: blocked"), false},
		{"401 does not trip", fmt.Errorf("wrap: %w", errors.New("unexpected status 401: nope")), false},
		{"context cancel does not trip", context.Canceled, false},
		{"oversized artifact does not trip", fetch.ErrArtifactTooLarge, false},
		{"locally held request does not trip", client.ErrUpstreamHeld, false},
		{"5xx upstream-down trips", fetch.ErrUpstreamDown, true},
		{"rate limited trips", fetch.ErrRateLimited, true},
		{"unexpected 5xx trips", errors.New("unexpected status 503: boom"), true},
		{"transport error trips", errors.New("dial tcp: connection refused"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tripsBreaker(tt.err); got != tt.want {
				t.Errorf("tripsBreaker(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// A run of 403 policy blocks must never open the breaker: every call reaches the
// base fetcher and the 403 is returned to the caller each time.
func TestBlocksDoNotOpenBreaker(t *testing.T) {
	base := &fakeFetcher{err: errors.New("unexpected status 403: blocked")}
	f := New(base)

	const attempts = 12
	for i := 0; i < attempts; i++ {
		_, err := f.FetchWithHeaders(context.Background(),
			"https://firewall.sonatype.app/npm/@sonatype/policy-demo/-/policy-demo-2.1.0.tgz", nil)
		if err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("attempt %d: got err %v, want the upstream 403", i, err)
		}
	}
	if base.count() != attempts {
		t.Fatalf("base called %d/%d times: breaker opened on 403 blocks", base.count(), attempts)
	}
}

// Genuine upstream unavailability (5xx) must open the breaker so later calls are
// short-circuited instead of hammering a dead upstream.
func TestUpstreamDownOpensBreaker(t *testing.T) {
	base := &fakeFetcher{err: fmt.Errorf("server error: %w", fetch.ErrUpstreamDown)}
	f := New(base)

	const attempts = 10
	for i := 0; i < attempts; i++ {
		_, _ = f.FetchWithHeaders(context.Background(), "https://up.example.test/pkg.tgz", nil)
	}
	if base.count() >= attempts {
		t.Fatalf("base called %d/%d times: breaker never opened on 5xx", base.count(), attempts)
	}
}

// A shared upstream host must not let one ecosystem's blocks affect another:
// blocks never open the breaker, so allowed fetches keep succeeding.
func TestAllowedFetchAfterBlocksSucceeds(t *testing.T) {
	base := &fakeFetcher{err: errors.New("unexpected status 403: blocked")}
	f := New(base)
	url := "https://firewall.sonatype.app/mvn/x/1.1.0/x-1.1.0.jar"
	for i := 0; i < 8; i++ {
		_, _ = f.FetchWithHeaders(context.Background(), url, nil)
	}
	// Now the upstream serves an allowed artifact.
	base.mu.Lock()
	base.err = nil
	base.art = &fetch.Artifact{Size: 3}
	base.mu.Unlock()

	art, err := f.FetchWithHeaders(context.Background(),
		"https://firewall.sonatype.app/mvn/x/1.0.0/x-1.0.0.jar", nil)
	if err != nil {
		t.Fatalf("allowed fetch after blocks failed: %v", err)
	}
	if art == nil {
		t.Fatal("allowed fetch returned nil artifact")
	}
}

// GetBreakerState reports the state of every breaker created so far, which the
// server publishes as metrics and in the health report.
func TestGetBreakerState(t *testing.T) {
	base := &fakeFetcher{err: fmt.Errorf("server error: %w", fetch.ErrUpstreamDown)}
	f := New(base)

	if states := f.GetBreakerState(); len(states) != 0 {
		t.Fatalf("states before any fetch = %v, want none", states)
	}

	const attempts = 10
	for i := 0; i < attempts; i++ {
		_, _ = f.FetchWithHeaders(context.Background(), "https://down.example.test/pkg.tgz", nil)
	}
	if got := f.GetBreakerState()["down.example.test"]; got != "open" {
		t.Errorf("state = %q, want open", got)
	}

	_, _ = f.FetchWithHeaders(context.Background(), "https://up.example.test/pkg.tgz", nil)
	if got := f.GetBreakerState()["up.example.test"]; got != "closed" {
		t.Errorf("state = %q, want closed", got)
	}
}
