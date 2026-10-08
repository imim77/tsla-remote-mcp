package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"tsla-remote-mcp/internal/store"

	"golang.org/x/oauth2"
)

func TestConnectAccountFlow(t *testing.T) {
	tokens := store.NewInMemoryStore()
	s := newTestService(tokens)
	s.httpClient.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
		if r.Form.Get("code") != "test-code" || r.Form.Get("audience") != "https://fleet.example.com" || r.Form.Get("redirect_uri") != s.config.RedirectURL {
			t.Error("code exchange is missing expected OAuth parameters")
		}
		return tokenResponse(200, `{"access_token":"test-access","refresh_token":"test-refresh","expires_in":3600}`), nil
	})
	h := NewHandler(s, slog.New(slog.NewTextHandler(io.Discard, nil)))
	start := httptest.NewRecorder()
	h.Start().ServeHTTP(start, httptest.NewRequest(http.MethodGet, "/auth/tsla", nil))
	if start.Code != http.StatusFound {
		t.Fatalf("start status = %d", start.Code)
	}
	authorize, err := url.Parse(start.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := authorize.Query().Get("state")
	if len(state) != 64 || authorize.Query().Get("redirect_uri") != s.config.RedirectURL {
		t.Fatal("invalid authorization URL")
	}
	result := start.Result()
	defer result.Body.Close()
	cookies := result.Cookies()
	if len(cookies) != 1 || cookies[0].Value != state || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].MaxAge != 600 {
		t.Fatal("incorrect OAuth state cookie, including HTTPS behind a proxy")
	}
	request := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code&state="+state, nil)
	request.AddCookie(cookies[0])
	callback := httptest.NewRecorder()
	h.Callback().ServeHTTP(callback, request)
	if callback.Code != http.StatusSeeOther || callback.Header().Get("Location") != "/auth/success" {
		t.Fatalf("callback: %d %s", callback.Code, callback.Body.String())
	}
	callbackResult := callback.Result()
	defer callbackResult.Body.Close()
	if cleared := callbackResult.Cookies(); len(cleared) != 1 || cleared[0].MaxAge >= 0 {
		t.Fatal("callback did not clear OAuth cookie")
	}
	token, err := s.Token(context.Background())
	if err != nil || token.AccessToken != "test-access" || token.RefreshToken != "test-refresh" {
		t.Fatalf("account tokens unavailable: %v", err)
	}
	home := httptest.NewRecorder()
	h.Home().ServeHTTP(home, httptest.NewRequest(http.MethodGet, "/", nil))
	if home.Code != 200 || !strings.Contains(home.Body.String(), "Tesla račun je povezan") {
		t.Fatal("home did not show connected account")
	}
	// A recreated application has no credentials because storage is in memory.
	restarted := newTestService(store.NewInMemoryStore())
	if _, err := restarted.Token(context.Background()); !errors.Is(err, ErrNotConnected) {
		t.Fatal("restart should require reconnecting Tesla")
	}
}

func TestCallbackRejectsInvalidInputWithoutTokenExchange(t *testing.T) {
	for _, test := range []struct{ name, query, cookie string }{
		{"missing cookie", "?code=test-code&state=expected", ""},
		{"wrong state", "?code=test-code&state=wrong", "expected"},
		{"missing state", "?code=test-code", "expected"},
		{"missing code", "?state=expected", "expected"},
		{"denied consent", "?error=access_denied&state=expected", "expected"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := newTestService(store.NewInMemoryStore())
			s.httpClient.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("invalid callback attempted exchange")
				return nil, nil
			})
			h := NewHandler(s, slog.New(slog.NewTextHandler(io.Discard, nil)))
			request := httptest.NewRequest(http.MethodGet, "/auth/callback"+test.query, nil)
			if test.cookie != "" {
				request.AddCookie(&http.Cookie{Name: stateCookie, Value: test.cookie})
			}
			response := httptest.NewRecorder()
			h.Callback().ServeHTTP(response, request)
			if response.Code != 400 {
				t.Fatalf("status = %d, want 400", response.Code)
			}
		})
	}
}

type failingTokenStore struct{}

func (failingTokenStore) Save(context.Context, *oauth2.Token) error {
	return errors.New("storage unavailable")
}
func (failingTokenStore) Load(context.Context) (*oauth2.Token, error) {
	return nil, store.ErrTokenNotFound
}

func TestCallbackDoesNotShowSuccessWhenSavingFails(t *testing.T) {
	s := newTestService(failingTokenStore{})
	s.httpClient.Transport = transportFunc(func(*http.Request) (*http.Response, error) {
		return tokenResponse(200, `{"access_token":"test-access","refresh_token":"test-refresh","expires_in":3600}`), nil
	})
	h := NewHandler(s, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/auth/callback?code=test-code&state=expected", nil)
	request.AddCookie(&http.Cookie{Name: stateCookie, Value: "expected"})
	response := httptest.NewRecorder()
	h.Callback().ServeHTTP(response, request)
	if response.Code != 500 || response.Header().Get("Location") != "" {
		t.Fatalf("failed save returned %d %s", response.Code, response.Header().Get("Location"))
	}
}
