package executor

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	libarazzo "github.com/pb33f/libopenapi/arazzo"
	higharazzo "github.com/pb33f/libopenapi/datamodel/high/arazzo"
	"github.com/pb33f/libopenapi/orderedmap"
)

func TestParamLocationPrefersArazzoIn(t *testing.T) {
	t.Parallel()
	got := paramLocation("Authorization", "/v1/asset-models", nil, map[string]string{
		"Authorization": "header",
	})
	if got != "header" {
		t.Fatalf("got %q, want header", got)
	}
}

func TestParamLocationDefaultsAuthorizationToHeader(t *testing.T) {
	t.Parallel()
	got := paramLocation("Authorization", "/v1/asset-models", nil, nil)
	if got != "header" {
		t.Fatalf("got %q, want header (must not fall through to query)", got)
	}
}

func TestParamLocationKeepsQueryWhenSpecified(t *testing.T) {
	t.Parallel()
	got := paramLocation("limit", "/v1/items", map[string]string{"limit": "query"}, nil)
	if got != "query" {
		t.Fatalf("got %q, want query", got)
	}
}

func TestArazzoParameterLocationsFromComponents(t *testing.T) {
	t.Parallel()
	params := orderedmap.New[string, *higharazzo.Parameter]()
	params.Set("authorization", &higharazzo.Parameter{
		Name: "Authorization",
		In:   "header",
	})
	locs := ArazzoParameterLocations(&higharazzo.Arazzo{
		Components: &higharazzo.Components{Parameters: params},
	})
	if locs["Authorization"] != "header" {
		t.Fatalf("got %#v", locs)
	}
}

func TestExecuteSendsAuthorizationAsHeader(t *testing.T) {
	t.Parallel()
	var gotAuth, gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `[]`)
	}))
	t.Cleanup(srv.Close)

	exec := New(srv.Client(), srv.URL, nil, nil, map[string]string{
		"Authorization": "header",
	})
	resp, err := exec.Execute(context.Background(), &libarazzo.ExecutionRequest{
		Method:        http.MethodGet,
		OperationPath: "/v1/asset-models",
		Parameters:    map[string]any{"Authorization": "Bearer test-token"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization header = %q", gotAuth)
	}
	if strings.Contains(gotURL, "Authorization") || strings.Contains(gotURL, "test-token") {
		t.Fatalf("token leaked into URL: %s", gotURL)
	}
}
