package sources

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestLooksLikeHTMLAndOpenAPI(t *testing.T) {
	t.Parallel()
	if !looksLikeHTML([]byte("<!doctype html><html></html>"), "text/html") {
		t.Fatal("expected HTML detection")
	}
	if !looksLikeOpenAPI([]byte("openapi: 3.0.3\ninfo:\n  title: x\n")) {
		t.Fatal("expected YAML OpenAPI detection")
	}
	if !looksLikeOpenAPI([]byte(`{"openapi":"3.0.3","info":{"title":"x"}}`)) {
		t.Fatal("expected JSON OpenAPI detection")
	}
}

func TestHTTPHandlerDoesNotPreferHTML(t *testing.T) {
	t.Parallel()
	spec := []byte("openapi: 3.0.3\ninfo:\n  title: x\n  version: '1'\npaths: {}\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if prefersHTML(r.Header.Get("Accept")) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html>docs</html>"))
			return
		}
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(spec)
	}))
	t.Cleanup(srv.Close)

	handler := NewHTTPHandler(srv.Client())
	body, err := handler(srv.URL + "/i/interface-7/user-session-api")
	if err != nil {
		t.Fatal(err)
	}
	if !looksLikeOpenAPI(body) {
		t.Fatalf("expected OpenAPI body, got %s", body)
	}
}

func TestHTTPHandlerRejectsHTML(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><html><head><title>docs</title></head><body>login</body></html>"))
	}))
	t.Cleanup(srv.Close)

	handler := NewHTTPHandler(srv.Client())
	_, err := handler(srv.URL + "/i/interface-7/user-session-api")
	if err == nil {
		t.Fatal("expected error on HTML response")
	}
	if !strings.Contains(err.Error(), "returned HTML") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func prefersHTML(accept string) bool {
	var htmlQ, starQ float64 = -1, -1
	var hasHTML bool
	for _, part := range strings.Split(accept, ",") {
		media, q := parseAcceptPart(part)
		switch media {
		case "text/html", "application/xhtml+xml":
			hasHTML = true
			if q > htmlQ {
				htmlQ = q
			}
		case "*/*":
			if q > starQ {
				starQ = q
			}
		}
	}
	if !hasHTML {
		return false
	}
	if starQ < 0 {
		return true
	}
	return htmlQ >= starQ
}

func parseAcceptPart(p string) (media string, q float64) {
	q = 1
	segs := strings.Split(p, ";")
	media = strings.TrimSpace(segs[0])
	for _, s := range segs[1:] {
		s = strings.TrimSpace(s)
		if after, ok := strings.CutPrefix(s, "q="); ok {
			if parsed, err := strconv.ParseFloat(after, 64); err == nil {
				q = parsed
			}
		}
	}
	return media, q
}

func looksLikeOpenAPI(body []byte) bool {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return false
	}
	if s[0] == '{' || s[0] == '[' {
		ls := strings.ToLower(s)
		return strings.Contains(ls, `"openapi"`) || strings.Contains(ls, `"swagger"`)
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		return strings.HasPrefix(lower, "openapi:") || strings.HasPrefix(lower, "swagger:")
	}
	return false
}
