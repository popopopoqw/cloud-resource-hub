package respool

import (
	"testing"
	"time"
)

func TestReserveReplySlotIsAtomic(t *testing.T) {
	replyMu.Lock()
	previous := lastReplyAt
	lastReplyAt = time.Time{}
	replyMu.Unlock()
	t.Cleanup(func() {
		replyMu.Lock()
		lastReplyAt = previous
		replyMu.Unlock()
	})

	if wait := reserveReplySlot(); wait != 0 {
		t.Fatalf("first reply should be allowed, got wait=%d", wait)
	}
	if wait := reserveReplySlot(); wait <= 0 {
		t.Fatal("second immediate reply should be rate limited")
	}
}

func TestForumReplyResult(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		code    string
		message string
		ok      bool
	}{
		{name: "object success", payload: `{"code":0,"message":"发布成功"}`, code: "0", message: "发布成功", ok: true},
		{name: "object error", payload: `{"code":"message","message":"两次发帖间隔太短"}`, code: "message", message: "两次发帖间隔太短", ok: true},
		{name: "array success", payload: `[0,"发布成功"]`, code: "0", message: "发布成功", ok: true},
		{name: "invalid", payload: `<html>error</html>`, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, message, ok := forumReplyResult([]byte(tt.payload))
			if code != tt.code || message != tt.message || ok != tt.ok {
				t.Fatalf("got (%q, %q, %v), want (%q, %q, %v)", code, message, ok, tt.code, tt.message, tt.ok)
			}
		})
	}
}
