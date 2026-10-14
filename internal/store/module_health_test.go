package store

import (
	"path/filepath"
	"testing"
	"time"

	"resource-center/internal/model"
)

func TestMemStoreModuleHealth(t *testing.T) {
	st := NewMemStore()
	checkedAt := time.Date(2026, 10, 12, 10, 30, 0, 0, time.Local)
	want := model.ModuleHealth{
		ModuleID: "drive", Available: true, Label: "可用",
		Detail: "123云盘可达", CheckedAt: checkedAt,
	}
	if err := st.SaveModuleHealth([]model.ModuleHealth{want}); err != nil {
		t.Fatal(err)
	}
	items, err := st.ListModuleHealth()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0] != want {
		t.Fatalf("items = %#v, want %#v", items, want)
	}
}

func TestSQLiteModuleHealthPersistsAndUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "health.db")
	st, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	checkedAt := time.Date(2026, 10, 12, 11, 0, 0, 0, time.UTC)
	if err := st.SaveModuleHealth([]model.ModuleHealth{
		{ModuleID: "drive", Available: true, Label: "可用", Detail: "第一次", CheckedAt: checkedAt},
		{ModuleID: "mailbox", Label: "异常", Detail: "连接失败", CheckedAt: checkedAt},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveModuleHealth([]model.ModuleHealth{
		{ModuleID: "drive", Label: "异常", Detail: "已更新", CheckedAt: checkedAt.Add(time.Minute)},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	items, err := reopened.ListModuleHealth()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if got := items[0]; got.ModuleID != "drive" || got.Available || got.Label != "异常" || got.Detail != "已更新" || !got.CheckedAt.Equal(checkedAt.Add(time.Minute)) {
		t.Fatalf("drive = %#v", got)
	}
}
