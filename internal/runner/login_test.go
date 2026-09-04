package runner

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func testdataFile(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", name)
}

func TestListAndDescribeWorkflows(t *testing.T) {
	t.Parallel()
	r := mustLoadLogin(t, httptest.NewServer(http.NotFoundHandler()))
	list := r.ListWorkflows()
	if len(list) != 1 || list[0].WorkflowID != "login-with-password" {
		t.Fatalf("unexpected workflows: %+v", list)
	}
	info, err := r.DescribeWorkflow("login-with-password")
	if err != nil {
		t.Fatal(err)
	}
	if len(info.StepIDs) != 4 {
		t.Fatalf("expected 4 steps, got %v", info.StepIDs)
	}
}

func TestLoginHappyPath204(t *testing.T) {
	t.Parallel()
	var sawCookie atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/login" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		body := decodeJSON(t, r)
		if body["email"] != "user@example.com" || body["password"] != "secret" {
			t.Errorf("unexpected credentials: %v", body)
		}
		http.SetCookie(w, &http.Cookie{Name: "ifc_session", Value: "id-token", Path: "/"})
		http.SetCookie(w, &http.Cookie{Name: "ifc_csrf", Value: "csrf-token", Path: "/"})
		sawCookie.Store(true)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	r := mustLoadLogin(t, srv)
	result := mustRunLogin(t, r, map[string]any{
		"email":    "user@example.com",
		"password": "secret",
		"baseUrl":  srv.URL,
	})
	if !result.Success {
		t.Fatalf("expected success, err=%v steps=%s", result.Error, stepSummary(result))
	}
	if len(result.Steps) != 1 || result.Steps[0].StepId != "submit-credentials" {
		t.Fatalf("expected only submit-credentials, got %s", stepSummary(result))
	}
	if !sawCookie.Load() {
		t.Fatal("login request was not observed")
	}
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if jar := r.CookieJar(); jar == nil {
		t.Fatal("missing cookie jar")
	} else if cookies := jar.Cookies(u); len(cookies) == 0 {
		t.Fatal("expected session cookies in the jar")
	}
}

func TestLoginNewPasswordChallenge(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/auth/login":
			writeJSON(w, http.StatusOK, map[string]any{
				"challengeName": "NEW_PASSWORD_REQUIRED",
				"session":       "sess-1",
				"parameters":    map[string]any{"userId": "abc"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/auth/login/challenge":
			body := decodeJSON(t, r)
			if body["challengeName"] != "NEW_PASSWORD_REQUIRED" {
				t.Errorf("unexpected challenge payload: %v", body)
			}
			responses, _ := body["responses"].(map[string]any)
			if responses["NEW_PASSWORD"] != "N3w-pass!" {
				t.Errorf("missing new password: %v", body)
			}
			if body["session"] != "sess-1" {
				t.Errorf("expected challenge session, got %v", body["session"])
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	r := mustLoadLogin(t, srv)
	result := mustRunLogin(t, r, map[string]any{
		"email":       "user@example.com",
		"password":    "old",
		"newPassword": "N3w-pass!",
		"baseUrl":     srv.URL,
	})
	if !result.Success {
		t.Fatalf("expected success, err=%v steps=%s", result.Error, stepSummary(result))
	}
	ids := stepIDs(result)
	if strings.Join(ids, ",") != "submit-credentials,answer-new-password" {
		t.Fatalf("unexpected steps: %s", strings.Join(ids, ","))
	}
}

func TestLoginSoftwareTokenMFA(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/auth/login":
			writeJSON(w, http.StatusOK, map[string]any{
				"challengeName": "SOFTWARE_TOKEN_MFA",
				"session":       "sess-mfa",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/auth/login/challenge":
			body := decodeJSON(t, r)
			if body["challengeName"] != "SOFTWARE_TOKEN_MFA" {
				t.Errorf("unexpected challenge: %v", body)
			}
			responses, _ := body["responses"].(map[string]any)
			if responses["SOFTWARE_TOKEN_MFA_CODE"] != "123456" {
				t.Errorf("missing totp: %v", body)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	r := mustLoadLogin(t, srv)
	result := mustRunLogin(t, r, map[string]any{
		"email":    "user@example.com",
		"password": "secret",
		"totpCode": "123456",
		"baseUrl":  srv.URL,
	})
	if !result.Success {
		t.Fatalf("expected success, err=%v steps=%s", result.Error, stepSummary(result))
	}
	ids := stepIDs(result)
	if strings.Join(ids, ",") != "submit-credentials,answer-totp-mfa" {
		t.Fatalf("unexpected steps: %s", strings.Join(ids, ","))
	}
}

func TestLoginRetryOn429ThenSucceed(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/login" {
			http.NotFound(w, r)
			return
		}
		n := attempts.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	r := mustLoadLogin(t, srv)
	result := mustRunLogin(t, r, map[string]any{
		"email":    "user@example.com",
		"password": "secret",
		"baseUrl":  srv.URL,
	})
	if !result.Success {
		t.Fatalf("expected success after retry, err=%v", result.Error)
	}
	if attempts.Load() < 2 {
		t.Fatalf("expected a retry, attempts=%d", attempts.Load())
	}
	var credentialSteps int
	for _, step := range result.Steps {
		if step.StepId == "submit-credentials" {
			credentialSteps++
		}
	}
	if credentialSteps < 2 {
		t.Fatalf("expected submit-credentials to run twice, steps=%s", stepSummary(result))
	}
}

func TestLoginAbortOn401(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"reason":"invalid_credentials"}`))
	}))
	t.Cleanup(srv.Close)

	r := mustLoadLogin(t, srv)
	result, err := r.ExecuteWorkflow(context.Background(), "login-with-password", map[string]any{
		"email":    "user@example.com",
		"password": "wrong",
		"baseUrl":  srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatal("expected auth failure")
	}
}

func TestCookieJarPersistsAcrossSteps(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/set":
			http.SetCookie(w, &http.Cookie{Name: "ifc_session", Value: "abc", Path: "/"})
			w.WriteHeader(http.StatusNoContent)
		case "/echo":
			c, err := r.Cookie("ifc_session")
			if err != nil {
				http.Error(w, "missing cookie", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"session": c.Value})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	r, err := FromFile(testdataFile(t, "cookie.arazzo.yaml"),
		WithHTTPClient(srv.Client()),
		WithFastRetries(),
		WithBaseURL(srv.URL),
		WithLogger(slog.New(slog.DiscardHandler)),
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.ExecuteWorkflow(context.Background(), "cookie-roundtrip", map[string]any{
		"baseUrl": srv.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("expected success, err=%v steps=%s", result.Error, stepSummary(result))
	}
	if result.Steps[1].Outputs["session"] != "abc" {
		t.Fatalf("cookie was not forwarded: %+v", result.Steps[1].Outputs)
	}
}

func TestRemoteOpenAPISource(t *testing.T) {
	t.Parallel()
	spec := []byte(`openapi: 3.0.0
info:
  title: test
  version: 1.0.0
paths:
  /auth/login:
    post:
      operationId: passwordLogin
      responses:
        "204":
          description: ok
  /auth/login/challenge:
    post:
      operationId: respondToLoginChallenge
      responses:
        "204":
          description: ok
`)
	mux := http.NewServeMux()
	mux.HandleFunc("/i/interface-7/user-session-api", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<!doctype html><html><body>docs</body></html>"))
			return
		}
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(spec)
	})
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	doc, err := os.ReadFile(testdataFile(t, "login.arazzo.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	rewritten := strings.Replace(
		string(doc),
		"url: https://ifc7.dev/i/interface-7/user-session-api",
		"url: "+srv.URL+"/i/interface-7/user-session-api",
		1,
	)
	tmp := filepath.Join(t.TempDir(), "login.arazzo.yaml")
	if err := os.WriteFile(tmp, []byte(rewritten), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := FromFile(tmp, WithHTTPClient(srv.Client()), WithFastRetries(), WithBaseURL(srv.URL), WithLogger(slog.New(slog.DiscardHandler)))
	if err != nil {
		t.Fatal(err)
	}
	result := mustRunLogin(t, r, map[string]any{
		"email":    "user@example.com",
		"password": "secret",
		"baseUrl":  srv.URL,
	})
	if !result.Success {
		t.Fatalf("remote OpenAPI source failed: %v", result.Error)
	}
}

func mustLoadLogin(t *testing.T, srv *httptest.Server) *Runner {
	t.Helper()
	r, err := FromFile(testdataFile(t, "login.arazzo.yaml"),
		WithHTTPClient(srv.Client()),
		WithFastRetries(),
		WithBaseURL(srv.URL),
		WithLogger(slog.New(slog.DiscardHandler)),
	)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func mustRunLogin(t *testing.T, r *Runner, inputs map[string]any) *Result {
	t.Helper()
	result, err := r.ExecuteWorkflow(context.Background(), "login-with-password", inputs)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func decodeJSON(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]any{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json: %v body=%s", err, b)
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, body map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func stepIDs(result *Result) []string {
	ids := make([]string, 0, len(result.Steps))
	for _, s := range result.Steps {
		ids = append(ids, s.StepId)
	}
	return ids
}

func stepSummary(result *Result) string {
	parts := make([]string, 0, len(result.Steps))
	for _, s := range result.Steps {
		err := ""
		if s.Error != nil {
			err = ": " + s.Error.Error()
		}
		parts = append(parts, s.StepId+err)
	}
	return strings.Join(parts, "; ")
}
