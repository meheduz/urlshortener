package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestApp(t *testing.T) *app {
	t.Helper()
	db, err := openDB(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &app{db: db, baseURL: "http://localhost:8080"}
}

func TestValidURL(t *testing.T) {
	valid := []string{
		"http://example.com",
		"https://example.com/some/very/long/url?q=1",
	}
	invalid := []string{
		"",
		"example.com",
		"ftp://example.com",
		"https://",
		"://nope",
	}
	for _, raw := range valid {
		if !validURL(raw) {
			t.Errorf("validURL(%q) = false, want true", raw)
		}
	}
	for _, raw := range invalid {
		if validURL(raw) {
			t.Errorf("validURL(%q) = true, want false", raw)
		}
	}
}

func TestGenerateCode(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		code, err := generateCode()
		if err != nil {
			t.Fatalf("generateCode: %v", err)
		}
		if len(code) != codeLength {
			t.Fatalf("code %q has length %d, want %d", code, len(code), codeLength)
		}
		if strings.ContainsFunc(code, func(r rune) bool { return !strings.ContainsRune(alphabet, r) }) {
			t.Fatalf("code %q contains characters outside the alphabet", code)
		}
		seen[code] = true
	}
	if len(seen) < 990 {
		t.Fatalf("only %d unique codes out of 1000, generation looks non-random", len(seen))
	}
}

func shorten(t *testing.T, a *app, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(body))
	rec := httptest.NewRecorder()
	a.routes().ServeHTTP(rec, req)
	return rec
}

func TestShortenCreatesURL(t *testing.T) {
	a := newTestApp(t)
	rec := shorten(t, a, `{"url":"https://example.com/some/very/long/url"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	var resp struct {
		ShortURL string `json:"short_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.HasPrefix(resp.ShortURL, "http://localhost:8080/") {
		t.Fatalf("short_url = %q, want the configured base url prefix", resp.ShortURL)
	}
	if code := strings.TrimPrefix(resp.ShortURL, "http://localhost:8080/"); len(code) != codeLength {
		t.Fatalf("code %q has length %d, want %d", code, len(code), codeLength)
	}
}

func TestShortenRejectsInvalidURL(t *testing.T) {
	a := newTestApp(t)
	if rec := shorten(t, a, `{"url":"not-a-url"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if rec := shorten(t, a, `not json`); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRedirectExistingCode(t *testing.T) {
	a := newTestApp(t)
	const original = "https://example.com/some/very/long/url"

	code, err := a.store(original)
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	rec := httptest.NewRecorder()
	a.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+code, nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if got := rec.Header().Get("Location"); got != original {
		t.Fatalf("Location = %q, want %q", got, original)
	}
}

func TestRedirectUnknownCode(t *testing.T) {
	a := newTestApp(t)
	rec := httptest.NewRecorder()
	a.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/zzzzz", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHealth(t *testing.T) {
	a := newTestApp(t)
	rec := httptest.NewRecorder()
	a.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"status":"ok"}` {
		t.Fatalf("body = %s, want {\"status\":\"ok\"}", got)
	}
}
