package gce

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/api/googleapi"
)

const fakeToken = "fake-token"

func newFakeTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token/", func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			AccessToken string    `json:"accessToken"`
			ExpireTime  time.Time `json:"expireTime"`
		}{
			AccessToken: fakeToken,
			ExpireTime:  time.Now().Add(5 * time.Minute),
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	return httptest.NewServer(mux)
}

func TestAltTokenSource(t *testing.T) {
	srv := newFakeTokenServer(t)
	defer srv.Close()

	alt := &AltTokenSource{
		oauthClient: srv.Client(),
		tokenURL:    srv.URL + "/token/",
		tokenBody:   `{"projectNumber":12345}`,
	}

	tkn, err := alt.Token()
	if err != nil {
		t.Fatalf("alt.Token() returned unexpected error: %v", err)
	}
	if tkn.AccessToken != fakeToken {
		t.Errorf("alt.Token() AccessToken = %q, want %q", tkn.AccessToken, fakeToken)
	}
}

func TestNewAltTokenSource_EndToEnd(t *testing.T) {
	const metadataSAToken = "metadata-sa-token"
	const wantBody = `{"projectNumber":67890,"clusterId":"test-cluster"}`

	var metaHits atomic.Int32
	metaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Metadata-Flavor"); got != "Google" {
			http.Error(w, "missing Metadata-Flavor header", http.StatusForbidden)
			return
		}
		if r.URL.Path != "/computeMetadata/v1/instance/service-accounts/default/token" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		metaHits.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": metadataSAToken,
			"expires_in":   3600,
			"token_type":   "Bearer",
		})
	}))
	defer metaSrv.Close()

	t.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(metaSrv.URL, "http://"))

	var gkeTokenHits atomic.Int32
	gkeTokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}
		if gotAuth := r.Header.Get("Authorization"); gotAuth != "Bearer "+metadataSAToken {
			http.Error(w, "unexpected Authorization: "+gotAuth, http.StatusUnauthorized)
			return
		}
		bodyBytes, _ := io.ReadAll(r.Body)
		if string(bodyBytes) != wantBody {
			http.Error(w, "unexpected body: "+string(bodyBytes), http.StatusBadRequest)
			return
		}
		gkeTokenHits.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accessToken": fakeToken,
			"expireTime":  time.Now().Add(10 * time.Minute),
		})
	}))
	defer gkeTokenSrv.Close()

	ts := NewAltTokenSource(context.Background(), gkeTokenSrv.URL+"/token/", wantBody)
	for i := 0; i < 2; i++ {
		tkn, err := ts.Token()
		if err != nil {
			t.Fatalf("NewAltTokenSource().Token() call %d returned unexpected error: %v", i+1, err)
		}
		if tkn.AccessToken != fakeToken {
			t.Errorf("NewAltTokenSource().Token() call %d AccessToken = %q, want %q", i+1, tkn.AccessToken, fakeToken)
		}
	}
	if got := metaHits.Load(); got != 1 {
		t.Errorf("metadata server hits = %d, want 1", got)
	}
	if got := gkeTokenHits.Load(); got != 1 {
		t.Errorf("GKE token server hits = %d, want 1", got)
	}
}

func TestAltTokenSource_ErrorCases(t *testing.T) {
	t.Run("non-2xx response from token endpoint returns googleapi.Error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "permission denied", http.StatusForbidden)
		}))
		defer srv.Close()

		alt := &AltTokenSource{
			oauthClient: srv.Client(),
			tokenURL:    srv.URL + "/token/",
			tokenBody:   `{}`,
		}
		_, err := alt.Token()
		if err == nil {
			t.Fatal("alt.Token() succeeded, want error on 403 response")
		}
		var gerr *googleapi.Error
		if !errors.As(err, &gerr) {
			t.Fatalf("alt.Token() error = %T (%v), want *googleapi.Error", err, err)
		}
		if gerr.Code != http.StatusForbidden {
			t.Errorf("googleapi.Error.Code = %d, want %d", gerr.Code, http.StatusForbidden)
		}
	})

	t.Run("malformed JSON response from token endpoint", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"accessToken": "incomplete-json"`))
		}))
		defer srv.Close()

		alt := &AltTokenSource{
			oauthClient: srv.Client(),
			tokenURL:    srv.URL + "/token/",
			tokenBody:   `{}`,
		}
		if _, err := alt.Token(); err == nil {
			t.Fatal("alt.Token() succeeded, want error on malformed JSON response")
		}
	})

	t.Run("cancelled context fails token refresh", func(t *testing.T) {
		srv := newFakeTokenServer(t)
		defer srv.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		alt := &AltTokenSource{
			ctx:         ctx,
			oauthClient: srv.Client(),
			tokenURL:    srv.URL + "/token/",
			tokenBody:   `{}`,
		}
		if _, err := alt.Token(); err == nil {
			t.Fatal("alt.Token() succeeded with cancelled context, want error")
		}
	})
}
