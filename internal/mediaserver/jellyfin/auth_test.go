package jellyfin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthenticateUsesAuthorizationHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "MediaBrowser ") {
			t.Errorf("Authorization = %q, want MediaBrowser scheme", got)
		}
		if got := r.Header.Get("X-Emby-Authorization"); got != "" {
			t.Errorf("deprecated X-Emby-Authorization header was sent: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"AccessToken":"token","User":{"Id":"user-id","Name":"meet"}}`))
	}))
	defer server.Close()

	flow := NewAuthFlow("device-id", nil)
	flow.httpClient = server.Client()
	result, err := flow.authenticate(context.Background(), server.URL, "meet", "password")
	if err != nil {
		t.Fatal(err)
	}
	if result.Token != "token" || result.UserID != "user-id" {
		t.Fatalf("unexpected authentication result: %+v", result)
	}
}
