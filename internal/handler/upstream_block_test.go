package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
)

// firewall403 builds the error the fetch/cache path returns when the upstream
// (e.g. Sonatype Firewall) rejects an artifact with a 403 policy block.
func firewall403(body string) error {
	return fmt.Errorf("fetching from upstream: %w", fmt.Errorf("unexpected status 403: %s", body))
}

func newTestProxy() *Proxy {
	return &Proxy{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func TestServeUpstreamBlock(t *testing.T) {
	jsonBody := `{"status":403,"title":"Sonatype Firewall Report","detail":"blocked"}`

	tests := []struct {
		name        string
		err         error
		wantHandled bool
		wantStatus  int
		wantBody    string
		wantCT      string
	}{
		{
			name:        "firewall json 403 is forwarded",
			err:         firewall403(jsonBody),
			wantHandled: true,
			wantStatus:  403,
			wantBody:    jsonBody,
			wantCT:      "application/json",
		},
		{
			name:        "plain-text 403 body keeps text content-type",
			err:         firewall403("nope"),
			wantHandled: true,
			wantStatus:  403,
			wantBody:    "nope",
			wantCT:      "text/plain; charset=utf-8",
		},
		{
			name:        "non-403 upstream status is not a block",
			err:         fmt.Errorf("fetching from upstream: %w", fmt.Errorf("unexpected status 500: boom")),
			wantHandled: false,
		},
		{
			name:        "not-found is not a block",
			err:         ErrUpstreamNotFound,
			wantHandled: false,
		},
	}

	p := newTestProxy()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handled := p.serveUpstreamBlock(rec, tt.err)
			if handled != tt.wantHandled {
				t.Fatalf("handled = %v, want %v", handled, tt.wantHandled)
			}
			if !tt.wantHandled {
				if rec.Body.Len() != 0 {
					t.Errorf("expected no body written, got %q", rec.Body.String())
				}
				return
			}
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if got := rec.Header().Get("Content-Type"); got != tt.wantCT {
				t.Errorf("content-type = %q, want %q", got, tt.wantCT)
			}
		})
	}
}

func TestServeArtifactErrorForwardsPolicyBlocks(t *testing.T) {
	p := newTestProxy()

	// A 403 block is forwarded verbatim, not turned into the generic 502 message.
	t.Run("forwards 403 block", func(t *testing.T) {
		body := `{"status":403,"title":"Sonatype Firewall Report"}`
		rec := httptest.NewRecorder()
		p.serveArtifactError(rec, firewall403(body), "failed to fetch package")
		if rec.Code != 403 {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
		if got := rec.Body.String(); got != body {
			t.Errorf("body = %q, want %q", got, body)
		}
	})

	// Anything else becomes a 502 with the handler's generic message.
	t.Run("generic error becomes 502", func(t *testing.T) {
		rec := httptest.NewRecorder()
		p.serveArtifactError(rec, fmt.Errorf("dial tcp: refused"), "failed to fetch crate")
		if rec.Code != 502 {
			t.Fatalf("status = %d, want 502", rec.Code)
		}
		if got := rec.Body.String(); got != "failed to fetch crate\n" {
			t.Errorf("body = %q, want %q", got, "failed to fetch crate\n")
		}
	})
}
