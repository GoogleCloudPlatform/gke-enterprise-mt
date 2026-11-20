package gce

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/googleapi"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2"
)

const (
	tokenRequestTimeout = 10 * time.Second
	earlyExpiry         = 225 * time.Second
)

// AltTokenSource implements oauth2.TokenSource to fetch a token from a GKE token endpoint.
type AltTokenSource struct {
	ctx         context.Context
	oauthClient *http.Client
	tokenURL    string
	tokenBody   string
}

// Token returns a token or an error.
func (a *AltTokenSource) Token() (*oauth2.Token, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.tokenURL, strings.NewReader(a.tokenBody))
	if err != nil {
		return nil, err
	}
	res, err := a.oauthClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if err := googleapi.CheckResponse(res); err != nil {
		return nil, err
	}
	var tok struct {
		AccessToken string    `json:"accessToken"`
		ExpireTime  time.Time `json:"expireTime"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tok); err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken: tok.AccessToken,
		Expiry:      tok.ExpireTime,
	}, nil
}

// NewAltTokenSource returns a new AltTokenSource. The context is used for all
// token refreshes. If the context is cancelled, all subsequent token refreshes
// will fail.
func NewAltTokenSource(ctx context.Context, tokenURL, tokenBody string) oauth2.TokenSource {
	client := oauth2.NewClient(ctx, google.ComputeTokenSource(""))
	client.Timeout = tokenRequestTimeout
	ts := &AltTokenSource{
		ctx:         ctx,
		oauthClient: client,
		tokenURL:    tokenURL,
		tokenBody:   tokenBody,
	}
	return oauth2.ReuseTokenSourceWithExpiry(nil, ts, earlyExpiry)
}
