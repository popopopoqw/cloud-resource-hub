package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"resource-center/internal/respool"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func TestForumProxyKeepsLoginSession(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		header := make(http.Header)
		header.Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodGet {
			header.Add("Set-Cookie", "bbs_sid=session-1; Path=/; HttpOnly")
			header.Add("Set-Cookie", "bbs_token=deleted; Path=/; Max-Age=0")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     header,
				Body:       io.NopCloser(strings.NewReader(`<html><body><form action="?user-login.htm" method="post"></form><script src="/view/app.js"></script></body></html>`)),
			}, nil
		}

		if cookie, err := r.Cookie("bbs_sid"); err != nil || cookie.Value != "session-1" {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     header,
				Body:       io.NopCloser(strings.NewReader("missing login session")),
			}, nil
		}
		header.Set("Content-Type", "application/json")
		header.Add("Set-Cookie", "bbs_token=token-1; Path=/; HttpOnly")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(strings.NewReader(`{"code":0,"message":"ok"}`)),
		}, nil
	})

	originalSources := respool.Sources
	respool.Sources = append(append([]respool.Source{}, originalSources...), respool.Source{
		ID: "proxytest", Name: "proxytest", Base: "https://forum.invalid",
	})
	defer func() { respool.Sources = originalSources }()

	pool := respool.New("", "", "", t.TempDir(), false)
	server := &Server{pool: pool, forumHTTP: &http.Client{Transport: transport}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/forum/proxy/{source}/{path...}", server.handleForumProxyAll)
	mux.HandleFunc("POST /api/forum/proxy/{source}/{path...}", server.handleForumProxyAll)

	firstRequest := httptest.NewRequest(http.MethodGet, "/api/forum/proxy/proxytest/user-login.htm", nil)
	firstRecorder := httptest.NewRecorder()
	mux.ServeHTTP(firstRecorder, firstRequest)
	firstResponse := firstRecorder.Result()
	body, _ := io.ReadAll(firstResponse.Body)
	_ = firstResponse.Body.Close()
	page := string(body)
	if !strings.Contains(page, `/api/forum/proxy/proxytest/view/app.js`) {
		t.Fatalf("forum asset URL was not rewritten: %s", page)
	}
	if pool.HasCookie() {
		t.Fatal("bbs_token=deleted must not mark the forum as logged in")
	}

	secondRequest := httptest.NewRequest(http.MethodPost, "/api/forum/proxy/proxytest/user-login.htm?user-login.htm", strings.NewReader("email=user&password=hash"))
	secondRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range firstResponse.Cookies() {
		if cookie.MaxAge >= 0 {
			secondRequest.AddCookie(cookie)
		}
	}
	secondRecorder := httptest.NewRecorder()
	mux.ServeHTTP(secondRecorder, secondRequest)
	secondResponse := secondRecorder.Result()
	responseBody, _ := io.ReadAll(secondResponse.Body)
	_ = secondResponse.Body.Close()
	if secondResponse.StatusCode != http.StatusOK || !strings.Contains(string(responseBody), `"code":0`) {
		t.Fatalf("unexpected login response: status=%d body=%s", secondResponse.StatusCode, responseBody)
	}
	if !pool.HasCookie() {
		t.Fatal("valid bbs_token was not persisted")
	}
	if session := pool.ProxySession("proxytest"); !strings.Contains(session, "bbs_sid=session-1") || !strings.Contains(session, "bbs_token=token-1") {
		t.Fatalf("incomplete persisted forum session: %q", session)
	}
}

func TestRewriteForumURLLeavesExternalResourcesAlone(t *testing.T) {
	got := rewriteForumURL("https://static.geetest.com/v4/gt4.js", "https://pan1.me/user-login.htm", "/api/forum/proxy/pan1me")
	if got != "https://static.geetest.com/v4/gt4.js" {
		t.Fatalf("external resource should not be rewritten: %s", got)
	}
}
