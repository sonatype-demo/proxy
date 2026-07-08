package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/git-pkgs/registries/fetch"
)

func TestErrUpstreamNotFoundWrapsFetchErrNotFound(t *testing.T) {
	if !errors.Is(ErrUpstreamNotFound, fetch.ErrNotFound) {
		t.Fatal("ErrUpstreamNotFound does not wrap fetch.ErrNotFound")
	}
}

func TestGetOrFetchArtifactFromURL_NotFound(t *testing.T) {
	proxy, _, _, fetcher := setupTestProxy(t)
	fetcher.fetchErr = fetch.ErrNotFound

	_, err := proxy.GetOrFetchArtifactFromURL(context.Background(),
		"maven", "org.example:missing", "1.0", "missing-1.0.jar",
		"http://upstream.test/org/example/missing/1.0/missing-1.0.jar")

	if !errors.Is(err, ErrUpstreamNotFound) {
		t.Fatalf("want ErrUpstreamNotFound, got %v", err)
	}
}

func TestMavenHandler_UpstreamNotFoundReturns404(t *testing.T) {
	proxy, _, _, fetcher := setupTestProxy(t)
	fetcher.fetchErr = fetch.ErrNotFound

	h := NewMavenHandler(proxy, "http://localhost", "http://upstream.test", "http://portal.test")
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/org/example/missing/1.0/missing-1.0.jar")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404 for missing upstream artifact, got %d", resp.StatusCode)
	}
}
