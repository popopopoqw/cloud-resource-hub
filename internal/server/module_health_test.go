package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"resource-center/internal/model"
	"resource-center/internal/store"
)

func TestHandleModuleHealthReadsCacheOnly(t *testing.T) {
	st := store.NewMemStore()
	checkedAt := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	if err := st.SaveModuleHealth([]model.ModuleHealth{
		{ModuleID: "drive", Available: true, Label: "可用", Detail: "数据库缓存", CheckedAt: checkedAt},
	}); err != nil {
		t.Fatal(err)
	}
	s := &Server{store: st}
	w := httptest.NewRecorder()
	s.handleModuleHealth(w, httptest.NewRequest(http.MethodGet, "/api/v1/module-health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var response struct {
		Data map[string]model.ModuleHealth `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if got := response.Data["drive"]; !got.Available || got.Detail != "数据库缓存" || !got.CheckedAt.Equal(checkedAt) {
		t.Fatalf("drive = %#v", got)
	}
	if got := response.Data["library"]; got.Label != "待检测" || !got.CheckedAt.IsZero() {
		t.Fatalf("library = %#v", got)
	}
}
