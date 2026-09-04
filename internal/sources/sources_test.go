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
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><html></html>"))
	}))
	t.Cleanup(srv.Close)

	handler := NewHTTPHandler(srv.Client())
	_, err := handler(srv.URL + "/docs")
	if err == nil || !strings.Contains(err.Error(), "returned HTML") {
		t.Fatalf("expected HTML error, got %v", err)
	}
}

// prefersHTML matches ifc locator negotiation: HTML wins when Accept lists
// text/html and does not list */* with a higher q.
func prefersHTML(accept string) bool {
	if accept == "" {
		return false
	}
	htmlQ, starQ, hasHTML := -1.0, -1.0, false
	for _, p := range strings.Split(accept, ",") {
		media, q := parseAcceptPart(p)
		switch {
		case strings.HasPrefix(media, "text/html"):
			hasHTML = true
			if htmlQ < 0 {
				htmlQ = q
			}
		case media == "*/*":
			if starQ < 0 {
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
