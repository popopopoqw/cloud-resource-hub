package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyBookRecordsKeepOriginalStorage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "records.db")
	oldDB, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = oldDB.Exec(`CREATE TABLE book_records (
		id TEXT PRIMARY KEY, user_id TEXT NOT NULL, book_id TEXT NOT NULL, title TEXT NOT NULL,
		authors TEXT NOT NULL DEFAULT '[]', format TEXT NOT NULL DEFAULT '', size_bytes INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'queued', r2_key TEXT NOT NULL DEFAULT '', error TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL, finished_at TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = oldDB.Exec(`INSERT INTO book_records(id,user_id,book_id,title,status,r2_key,created_at)
		VALUES ('local','user','a','Local','done','books/user/a.epub','2026-01-01T00:00:00Z'),
		('remote','user','b','Remote','done','books/user/b.epub','2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	oldDB.Close()
	localPath := filepath.Join(dir, "books", "user", "a.epub")
	if err := os.MkdirAll(filepath.Dir(localPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, []byte("book"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.BackfillLegacyBookRecords("old-bucket", dir); err != nil {
		t.Fatal(err)
	}
	local, err := st.GetBookRecord("user", "local")
	if err != nil || local.StorageKind != "local" || local.StorageBucket != "" {
		t.Fatalf("local=%+v err=%v", local, err)
	}
	remote, err := st.GetBookRecord("user", "remote")
	if err != nil || remote.StorageKind != "r2" || remote.StorageBucket != "old-bucket" {
		t.Fatalf("remote=%+v err=%v", remote, err)
	}
}
