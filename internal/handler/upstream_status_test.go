package handler

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestUpstreamStatus(t *testing.T) {
	firewallBody := `{"status":403,"title":"Sonatype Firewall Report","detail":"Sonatype has identified this component as potentially malicious and blocked the download."}`

	tests := []struct {
		name     string
		err      error
		wantCode int
		wantBody string
		wantOK   bool
	}{
		{
			name:     "firewall 403 wrapped by fetchAndCacheFromURL",
			err:      fmt.Errorf("fetching from upstream: %w", fmt.Errorf("unexpected status 403: %s", firewallBody)),
			wantCode: http.StatusForbidden,
			wantBody: firewallBody,
			wantOK:   true,
		},
		{
			name:     "plain unexpected status 403",
			err:      fmt.Errorf("unexpected status 403: forbidden"),
			wantCode: http.StatusForbidden,
			wantBody: "forbidden",
			wantOK:   true,
		},
		{
			name:     "500 with empty body",
			err:      fmt.Errorf("unexpected status 500: "),
			wantCode: http.StatusInternalServerError,
			wantBody: "",
			wantOK:   true,
		},
		{
			name:   "not-found sentinel carries no status",
			err:    ErrUpstreamNotFound,
			wantOK: false,
		},
		{
			name:   "transport error carries no status",
			err:    errors.New("dial tcp: connection refused"),
			wantOK: false,
		},
		{
			name:   "nil error",
			err:    nil,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, body, ok := UpstreamStatus(tt.err)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if code != tt.wantCode {
				t.Errorf("code = %d, want %d", code, tt.wantCode)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}
