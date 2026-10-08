package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"tsla-remote-mcp/internal/store"

	"golang.org/x/oauth2"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newTestService(tokens store.Repository) *Service {
	return NewService(&oauth2.Config{
		ClientID: "test-client", ClientSecret: "test-secret",
		RedirectURL: "https://app.example.com/auth/callback",
		Endpoint:    oauth2.Endpoint{AuthURL: "https://auth.example.com/authorize", TokenURL: "https://auth.example.com/token", AuthStyle: oauth2.AuthStyleInParams},
	}, "https://fleet.example.com", tokens)
}

func tokenResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestTokenRefreshIsSerializedAndRotatedTokenIsSaved(t *testing.T) {
	tokens := store.NewInMemoryStore()
	if err := tokens.Save(context.Background(), &oauth2.Token{AccessToken: "old-access", RefreshToken: "old-refresh", Expiry: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	s := newTestService(tokens)
	var calls atomic.Int32
	s.httpClient.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		call := calls.Add(1)
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
		want := "old-refresh"
		if call == 2 {
			want = "new-refresh"
		}
		if r.Method != http.MethodPost || r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != want || r.Form.Get("client_id") != "test-client" {
			t.Errorf("unexpected refresh request: method %s, grant %s, refresh token matched %v", r.Method, r.Form.Get("grant_type"), r.Form.Get("refresh_token") == want)
		}
		return tokenResponse(200, `{"access_token":"new-access","refresh_token":"new-refresh","token_type":"Bearer","expires_in":3600}`), nil
	})
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			token, err := s.Token(context.Background())
			if err != nil || token.AccessToken != "new-access" {
				t.Errorf("Token() failed: %v", err)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("refresh requests = %d, want 1", calls.Load())
	}
	saved, err := tokens.Load(context.Background())
	if err != nil || saved.RefreshToken != "new-refresh" {
		t.Fatalf("rotated token was not saved: %v", err)
	}
	saved.Expiry = time.Now().Add(-time.Hour)
	if err := tokens.Save(context.Background(), saved); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("refresh requests = %d, want 2", calls.Load())
	}
}

func TestTokenFailures(t *testing.T) {
	for _, test := range []struct {
		name     string
		token    *oauth2.Token
		status   int
		body     string
		want     error
		wantCall bool
	}{
		{name: "not connected", want: ErrNotConnected},
		{name: "no refresh token", token: &oauth2.Token{AccessToken: "old", Expiry: time.Now().Add(-time.Hour)}, want: ErrReconnectRequired},
		{name: "revoked refresh", token: &oauth2.Token{AccessToken: "old", RefreshToken: "refresh", Expiry: time.Now().Add(-time.Hour)}, status: 400, body: `{"error":"invalid_grant"}`, want: ErrReconnectRequired, wantCall: true},
		{name: "Tesla unavailable", token: &oauth2.Token{AccessToken: "old", RefreshToken: "refresh", Expiry: time.Now().Add(-time.Hour)}, status: 503, body: `{"error":"temporarily_unavailable"}`, wantCall: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			tokens := store.NewInMemoryStore()
			if test.token != nil {
				if err := tokens.Save(context.Background(), test.token); err != nil {
					t.Fatal(err)
				}
			}
			s := newTestService(tokens)
			calls := 0
			s.httpClient.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return tokenResponse(test.status, test.body), nil
			})
			_, err := s.Token(context.Background())
			if err == nil || (test.want != nil && !errors.Is(err, test.want)) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if (calls > 0) != test.wantCall {
				t.Fatalf("refresh calls = %d", calls)
			}
			if test.token != nil {
				saved, loadErr := tokens.Load(context.Background())
				if loadErr != nil || saved.AccessToken != test.token.AccessToken || saved.RefreshToken != test.token.RefreshToken {
					t.Fatal("failed refresh overwrote stored credentials")
				}
			}
			if test.want == ErrReconnectRequired {
				connected, err := s.Connected(context.Background())
				if err != nil || connected {
					t.Fatal("invalid connection should request reconnection")
				}
			}
		})
	}
}

func TestTokenCancellationWhileWaiting(t *testing.T) {
	s := newTestService(store.NewInMemoryStore())
	if err := s.lock(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { <-s.gate }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Token(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestRefreshPersistsTokensWhenCallerDisconnectsAfterResponse(t *testing.T) {
	tokens := store.NewInMemoryStore()
	if err := tokens.Save(context.Background(), &oauth2.Token{AccessToken: "old", RefreshToken: "old-refresh", Expiry: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	s := newTestService(tokens)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.httpClient.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		cancel()
		return tokenResponse(200, `{"access_token":"new-access","refresh_token":"rotated-refresh","expires_in":3600}`), nil
	})
	if _, err := s.Token(ctx); err != nil {
		t.Fatal(err)
	}
	saved, err := tokens.Load(context.Background())
	if err != nil || saved.RefreshToken != "rotated-refresh" {
		t.Fatal("lost rotated token after cancellation")
	}
}

func TestReconnectReplacesRejectedCredentials(t *testing.T) {
	tokens := store.NewInMemoryStore()
	if err := tokens.Save(context.Background(), &oauth2.Token{AccessToken: "old", RefreshToken: "revoked", Expiry: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	s := newTestService(tokens)
	s.httpClient.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
		if r.Form.Get("grant_type") == "refresh_token" {
			return tokenResponse(400, `{"error":"invalid_grant"}`), nil
		}
		return tokenResponse(200, `{"access_token":"reconnected","refresh_token":"new-refresh","expires_in":3600}`), nil
	})
	if _, err := s.Token(context.Background()); !errors.Is(err, ErrReconnectRequired) {
		t.Fatalf("error = %v", err)
	}
	if _, err := s.Exchange(context.Background(), "new-code"); err != nil {
		t.Fatal(err)
	}
	token, err := s.Token(context.Background())
	if err != nil || token.AccessToken != "reconnected" {
		t.Fatalf("reconnect did not replace credentials: %v", err)
	}
	connected, err := s.Connected(context.Background())
	if err != nil || !connected {
		t.Fatal("reconnected account should be connected")
	}
}
