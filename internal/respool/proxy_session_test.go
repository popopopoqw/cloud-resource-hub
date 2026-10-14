package respool

import (
	"strings"
	"testing"
)

func TestValidForumCookieRejectsDeletedToken(t *testing.T) {
	if validForumCookie("cookie_test=deleted; bbs_token=deleted") {
		t.Fatal("deleted token must not be treated as a valid login")
	}
	if !validForumCookie("bbs_sid=sid; bbs_token=token-value") {
		t.Fatal("non-empty bbs_token should be treated as a valid login")
	}
}

func TestProxySessionMergesAndDeletesCookies(t *testing.T) {
	fetcher := New("", "", "", t.TempDir(), false)
	fetcher.MergeProxySession("pan1me", "bbs_sid=session-1; Path=/; HttpOnly")
	fetcher.MergeProxySession("pan1me", "cookie_test=test-1")
	fetcher.MergeProxySession("pan1me", "bbs_token=token-1")

	session := fetcher.ProxySession("pan1me")
	for _, want := range []string{"bbs_sid=session-1", "cookie_test=test-1", "bbs_token=token-1"} {
		if !strings.Contains(session, want) {
			t.Fatalf("proxy session %q does not contain %q", session, want)
		}
	}

	fetcher.MergeProxySession("pan1me", "bbs_token=deleted")
	if strings.Contains(fetcher.ProxySession("pan1me"), "bbs_token=") {
		t.Fatal("deleted bbs_token should be removed from the proxy session")
	}
	if fetcher.PersistProxySession("pan1me") {
		t.Fatal("a session without a valid bbs_token must not be persisted")
	}
}
