package gmail

import (
	"net/url"
	"testing"
)

func TestAuthorizationURLIncludesOfflineConsentAndState(t *testing.T) {
	raw := AuthorizationURL("client-id", "http://127.0.0.1:8080/callback", "state-value", "person@example.com")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	for key, want := range map[string]string{
		"client_id": "client-id", "redirect_uri": "http://127.0.0.1:8080/callback",
		"state": "state-value", "login_hint": "person@example.com",
		"access_type": "offline", "prompt": "consent", "scope": oauthScope,
	} {
		if got := query.Get(key); got != want {
			t.Errorf("%s=%q want %q", key, got, want)
		}
	}
}
