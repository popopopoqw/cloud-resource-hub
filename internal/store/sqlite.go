package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // 免 CGO 的 SQLite 驱动

	"resource-center/internal/model"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
  id         TEXT PRIMARY KEY,
  username   TEXT NOT NULL UNIQUE,
  pass_hash  TEXT NOT NULL,
  salt       TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  token      TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL,
  expires_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS tasks (
  id            TEXT PRIMARY KEY,
  user_id       TEXT NOT NULL,
  provider      TEXT NOT NULL DEFAULT '',
  share_url     TEXT NOT NULL DEFAULT '',
  share_name    TEXT NOT NULL DEFAULT '',
  file_ids      TEXT NOT NULL DEFAULT '[]',
  file_count    INTEGER NOT NULL DEFAULT 0,
  total_size    INTEGER NOT NULL DEFAULT 0,
  target_folder TEXT NOT NULL DEFAULT '',
  account_id    TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'queued',
  progress      INTEGER NOT NULL DEFAULT 0,
  error         TEXT NOT NULL DEFAULT '',
  transferred   TEXT NOT NULL DEFAULT '[]',
  created_at    TEXT NOT NULL,
  finished_at   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_tasks_user ON tasks(user_id, created_at DESC);
CREATE TABLE IF NOT EXISTS accounts (
  id             TEXT PRIMARY KEY,
  user_id        TEXT NOT NULL,
  provider       TEXT NOT NULL,
  name           TEXT NOT NULL,
  masked_secret  TEXT NOT NULL DEFAULT '',
  status         TEXT NOT NULL DEFAULT 'active',
  capacity_used  INTEGER NOT NULL DEFAULT 0,
  capacity_total INTEGER NOT NULL DEFAULT 0,
  created_at     TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS shelf_items (
  id        TEXT PRIMARY KEY,
  user_id   TEXT NOT NULL,
  book_id   TEXT NOT NULL,
  title     TEXT NOT NULL,
  authors   TEXT NOT NULL DEFAULT '[]',
  cover     TEXT NOT NULL DEFAULT '',
  format    TEXT NOT NULL DEFAULT '',
  size_text TEXT NOT NULL DEFAULT '',
  added_at  TEXT NOT NULL,
  UNIQUE(user_id, book_id)
);
CREATE TABLE IF NOT EXISTS book_records (
  id          TEXT PRIMARY KEY,
  user_id     TEXT NOT NULL,
  book_id     TEXT NOT NULL,
  title       TEXT NOT NULL,
  authors     TEXT NOT NULL DEFAULT '[]',
  format      TEXT NOT NULL DEFAULT '',
  size_bytes  INTEGER NOT NULL DEFAULT 0,
  status      TEXT NOT NULL DEFAULT 'queued',
  r2_key      TEXT NOT NULL DEFAULT '',
  storage_kind TEXT NOT NULL DEFAULT '',
  storage_bucket TEXT NOT NULL DEFAULT '',
  error       TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL,
  finished_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_book_records_user ON book_records(user_id, created_at DESC);
CREATE TABLE IF NOT EXISTS integration_settings (
  section    TEXT NOT NULL,
  key        TEXT NOT NULL,
  value      TEXT NOT NULL,
  is_secret  INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL,
  PRIMARY KEY(section, key)
);
CREATE TABLE IF NOT EXISTS module_health (
  module_id  TEXT PRIMARY KEY,
  available  INTEGER NOT NULL DEFAULT 0,
  label      TEXT NOT NULL DEFAULT '',
  detail     TEXT NOT NULL DEFAULT '',
  checked_at TEXT NOT NULL
);
`

// SQLiteStore SQLite 持久化实现（modernc 驱动，免 CGO）。
// 写入量小的单实例场景：单连接串行写，避免 SQLITE_BUSY。
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore 打开（或创建）数据库并建表。path 如 ./resource-center.db。
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化表结构失败: %w", err)
	}
	if err := migrateBookStorageColumns(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("升级书籍转存记录失败: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func migrateBookStorageColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(book_records)`)
	if err != nil {
		return err
	}
	columns := map[string]bool{}
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		columns[name] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !columns["storage_kind"] {
		if _, err := db.Exec(`ALTER TABLE book_records ADD COLUMN storage_kind TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columns["storage_bucket"] {
		if _, err := db.Exec(`ALTER TABLE book_records ADD COLUMN storage_bucket TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	return nil
}

// BackfillLegacyBookRecords 在切换默认 Bucket 前标记旧记录的原存储位置。
func (s *SQLiteStore) BackfillLegacyBookRecords(oldBucket, dataDir string) error {
	rows, err := s.db.Query(`SELECT id, r2_key FROM book_records WHERE status='done' AND r2_key<>'' AND storage_kind=''`)
	if err != nil {
		return err
	}
	type legacyRecord struct{ id, key string }
	var records []legacyRecord
	for rows.Next() {
		var record legacyRecord
		if err := rows.Scan(&record.id, &record.key); err != nil {
			rows.Close()
			return err
		}
		records = append(records, record)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, record := range records {
		kind, bucket := "r2", oldBucket
		if oldBucket == "" {
			kind, bucket = "local", ""
		} else if !strings.Contains(record.key, "..") && !strings.HasPrefix(record.key, "/") {
			if info, err := os.Stat(filepath.Join(dataDir, filepath.FromSlash(record.key))); err == nil && info.Mode().IsRegular() {
				kind, bucket = "local", ""
			}
		}
		if _, err := s.db.Exec(`UPDATE book_records SET storage_kind=?, storage_bucket=? WHERE id=? AND storage_kind=''`, kind, bucket, record.id); err != nil {
			return err
		}
	}
	return nil
}

// Close 关闭数据库。
func (s *SQLiteStore) Close() error { return s.db.Close() }

// SaveIntegrationSettings 在一个事务中写入指定配置分组。
func (s *SQLiteStore) SaveIntegrationSettings(section string, items []model.IntegrationSetting) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range items {
		if item.Section != section {
			return fmt.Errorf("配置分组不一致")
		}
		if _, err := tx.Exec(`INSERT INTO integration_settings(section,key,value,is_secret,updated_at)
			VALUES(?,?,?,?,?) ON CONFLICT(section,key) DO UPDATE SET
			value=excluded.value,is_secret=excluded.is_secret,updated_at=excluded.updated_at`,
			item.Section, item.Key, item.Value, item.Secret, fmtTime(item.UpdatedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListIntegrationSettings 返回全部持久化配置。
func (s *SQLiteStore) ListIntegrationSettings() ([]model.IntegrationSetting, error) {
	rows, err := s.db.Query(`SELECT section,key,value,is_secret,updated_at FROM integration_settings ORDER BY section,key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.IntegrationSetting
	for rows.Next() {
		var item model.IntegrationSetting
		var updated string
		if err := rows.Scan(&item.Section, &item.Key, &item.Value, &item.Secret, &updated); err != nil {
			return nil, err
		}
		item.UpdatedAt = parseTime(updated)
		out = append(out, item)
	}
	return out, rows.Err()
}

// SaveModuleHealth 在一个事务中写入模块最近一次探测结果。
func (s *SQLiteStore) SaveModuleHealth(items []model.ModuleHealth) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range items {
		if _, err := tx.Exec(`INSERT INTO module_health(module_id,available,label,detail,checked_at)
			VALUES(?,?,?,?,?) ON CONFLICT(module_id) DO UPDATE SET
			available=excluded.available,label=excluded.label,detail=excluded.detail,checked_at=excluded.checked_at`,
			item.ModuleID, item.Available, item.Label, item.Detail, fmtTime(item.CheckedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListModuleHealth 返回全部模块最近一次探测结果。
func (s *SQLiteStore) ListModuleHealth() ([]model.ModuleHealth, error) {
	rows, err := s.db.Query(`SELECT module_id,available,label,detail,checked_at FROM module_health ORDER BY module_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.ModuleHealth, 0)
	for rows.Next() {
		var item model.ModuleHealth
		var checked string
		if err := rows.Scan(&item.ModuleID, &item.Available, &item.Label, &item.Detail, &checked); err != nil {
			return nil, err
		}
		item.CheckedAt = parseTime(checked)
		out = append(out, item)
	}
	return out, rows.Err()
}

// ---- 工具 ----

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func parseTime(v string) time.Time {
	if v == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, v)
	return t
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func jsonStrings(v string) []string {
	var out []string
	_ = json.Unmarshal([]byte(v), &out)
	return out
}

// ---- 用户与会话 ----

// CreateUser 新增用户。
func (s *SQLiteStore) CreateUser(u *model.User) error {
	_, err := s.db.Exec(`INSERT INTO users(id, username, pass_hash, salt, created_at) VALUES(?,?,?,?,?)`,
		u.ID, u.Username, u.PassHash, u.Salt, fmtTime(u.CreatedAt))
	if isUniqueErr(err) {
		return ErrDupUser
	}
	return err
}

// UserByUsername 按用户名查询。
func (s *SQLiteStore) UserByUsername(username string) (*model.User, error) {
	row := s.db.QueryRow(`SELECT id, username, pass_hash, salt, created_at FROM users WHERE username=?`, username)
	var u model.User
	var created string
	if err := row.Scan(&u.ID, &u.Username, &u.PassHash, &u.Salt, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.CreatedAt = parseTime(created)
	return &u, nil
}

// UserByID 按 ID 查询。
func (s *SQLiteStore) UserByID(id string) (*model.User, error) {
	row := s.db.QueryRow(`SELECT id, username, pass_hash, salt, created_at FROM users WHERE id=?`, id)
	var u model.User
	var created string
	if err := row.Scan(&u.ID, &u.Username, &u.PassHash, &u.Salt, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.CreatedAt = parseTime(created)
	return &u, nil
}

// SaveSession 保存会话。
func (s *SQLiteStore) SaveSession(sess *model.Session) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO sessions(token, user_id, expires_at) VALUES(?,?,?)`,
		sess.Token, sess.UserID, fmtTime(sess.ExpiresAt))
	return err
}

// SessionByToken 查询会话。
func (s *SQLiteStore) SessionByToken(token string) (*model.Session, error) {
	row := s.db.QueryRow(`SELECT token, user_id, expires_at FROM sessions WHERE token=?`, token)
	var sess model.Session
	var exp string
	if err := row.Scan(&sess.Token, &sess.UserID, &exp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	sess.ExpiresAt = parseTime(exp)
	return &sess, nil
}

// DeleteSession 注销会话。
func (s *SQLiteStore) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token=?`, token)
	return err
}

// ---- 任务 ----

// CreateTask 新增任务。
func (s *SQLiteStore) CreateTask(t *model.Task) error {
	_, err := s.db.Exec(`INSERT INTO tasks
		(id, user_id, provider, share_url, share_name, file_ids, file_count, total_size,
		 target_folder, account_id, status, progress, error, transferred, created_at, finished_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.UserID, t.Provider, t.ShareURL, t.ShareName, mustJSON(t.FileIDs), t.FileCount,
		t.TotalSize, t.TargetFolder, t.AccountID, t.Status, t.Progress, t.Error,
		mustJSON(t.Transferred), fmtTime(t.CreatedAt), fmtTime(t.FinishedAt))
	return err
}

// UpdateTask 更新任务。
func (s *SQLiteStore) UpdateTask(t *model.Task) error {
	res, err := s.db.Exec(`UPDATE tasks SET provider=?, share_url=?, share_name=?, file_ids=?,
		file_count=?, total_size=?, target_folder=?, account_id=?, status=?, progress=?, error=?,
		transferred=?, created_at=?, finished_at=? WHERE id=? AND user_id=?`,
		t.Provider, t.ShareURL, t.ShareName, mustJSON(t.FileIDs), t.FileCount, t.TotalSize,
		t.TargetFolder, t.AccountID, t.Status, t.Progress, t.Error, mustJSON(t.Transferred),
		fmtTime(t.CreatedAt), fmtTime(t.FinishedAt), t.ID, t.UserID)
	if err != nil {
		return err
	}
	return rowsAffected(res)
}

func scanTask(row interface{ Scan(...any) error }) (*model.Task, error) {
	var t model.Task
	var fileIDs, transferred, created, finished string
	err := row.Scan(&t.ID, &t.UserID, &t.Provider, &t.ShareURL, &t.ShareName, &fileIDs,
		&t.FileCount, &t.TotalSize, &t.TargetFolder, &t.AccountID, &t.Status, &t.Progress,
		&t.Error, &transferred, &created, &finished)
	if err != nil {
		return nil, err
	}
	t.FileIDs = jsonStrings(fileIDs)
	_ = json.Unmarshal([]byte(transferred), &t.Transferred)
	t.CreatedAt = parseTime(created)
	t.FinishedAt = parseTime(finished)
	return &t, nil
}

const taskCols = `id, user_id, provider, share_url, share_name, file_ids, file_count, total_size,
 target_folder, account_id, status, progress, error, transferred, created_at, finished_at`

// GetTask 查询任务（校验归属）。
func (s *SQLiteStore) GetTask(userID, id string) (*model.Task, error) {
	t, err := scanTask(s.db.QueryRow(`SELECT `+taskCols+` FROM tasks WHERE id=? AND user_id=?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

// ListTasks 分页列出任务（倒序）。status 为空时不过滤。
func (s *SQLiteStore) ListTasks(userID, status string, page, pageSize int) ([]*model.Task, int, error) {
	where, args := `WHERE user_id=?`, []any{userID}
	if status != "" {
		where += ` AND status=?`
		args = append(args, status)
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(`SELECT `+taskCols+` FROM tasks `+where+
		` ORDER BY created_at DESC LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := make([]*model.Task, 0)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, t)
	}
	return list, total, rows.Err()
}

// DeleteTask 删除任务。
func (s *SQLiteStore) DeleteTask(userID, id string) error {
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return err
	}
	return rowsAffected(res)
}

// ---- 账号 ----

// SaveAccount 保存账号。
func (s *SQLiteStore) SaveAccount(a *model.Account) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO accounts
		(id, user_id, provider, name, masked_secret, status, capacity_used, capacity_total, created_at)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		a.ID, a.UserID, a.Provider, a.Name, a.MaskedSecret, a.Status,
		a.CapacityUsed, a.CapacityTotal, fmtTime(a.CreatedAt))
	return err
}

// ListAccounts 列出账号。
func (s *SQLiteStore) ListAccounts(userID string) ([]*model.Account, error) {
	rows, err := s.db.Query(`SELECT id, user_id, provider, name, masked_secret, status,
		capacity_used, capacity_total, created_at FROM accounts WHERE user_id=? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]*model.Account, 0)
	for rows.Next() {
		var a model.Account
		var created string
		if err := rows.Scan(&a.ID, &a.UserID, &a.Provider, &a.Name, &a.MaskedSecret, &a.Status,
			&a.CapacityUsed, &a.CapacityTotal, &created); err != nil {
			return nil, err
		}
		a.CreatedAt = parseTime(created)
		list = append(list, &a)
	}
	return list, rows.Err()
}

// DeleteAccount 删除账号。
func (s *SQLiteStore) DeleteAccount(userID, id string) error {
	res, err := s.db.Exec(`DELETE FROM accounts WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return err
	}
	return rowsAffected(res)
}

// ---- 书架 ----

// SaveShelfItem 收藏（UNIQUE(user_id, book_id) 保证幂等）。
func (s *SQLiteStore) SaveShelfItem(it *model.ShelfItem) error {
	_, err := s.db.Exec(`INSERT INTO shelf_items(id, user_id, book_id, title, authors, cover, format, size_text, added_at)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(user_id, book_id) DO UPDATE SET
		  title=excluded.title, authors=excluded.authors, cover=excluded.cover,
		  format=excluded.format, size_text=excluded.size_text, added_at=excluded.added_at`,
		it.ID, it.UserID, it.BookID, it.Title, mustJSON(it.Authors), it.Cover, it.Format, it.SizeText, fmtTime(it.AddedAt))
	return err
}

// ListShelf 列出书架。
func (s *SQLiteStore) ListShelf(userID string) ([]*model.ShelfItem, error) {
	rows, err := s.db.Query(`SELECT id, user_id, book_id, title, authors, cover, format, size_text, added_at
		FROM shelf_items WHERE user_id=? ORDER BY added_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]*model.ShelfItem, 0)
	for rows.Next() {
		var it model.ShelfItem
		var authors, added string
		if err := rows.Scan(&it.ID, &it.UserID, &it.BookID, &it.Title, &authors, &it.Cover,
			&it.Format, &it.SizeText, &added); err != nil {
			return nil, err
		}
		it.Authors = jsonStrings(authors)
		it.AddedAt = parseTime(added)
		list = append(list, &it)
	}
	return list, rows.Err()
}

// DeleteShelfItem 取消收藏（按 bookID）。
func (s *SQLiteStore) DeleteShelfItem(userID, bookID string) error {
	res, err := s.db.Exec(`DELETE FROM shelf_items WHERE user_id=? AND book_id=?`, userID, bookID)
	if err != nil {
		return err
	}
	return rowsAffected(res)
}

// ---- 书籍转存记录 ----

// CreateBookRecord 新增记录。
func (s *SQLiteStore) CreateBookRecord(r *model.BookRecord) error {
	_, err := s.db.Exec(`INSERT INTO book_records
		(id, user_id, book_id, title, authors, format, size_bytes, status, r2_key, storage_kind, storage_bucket, error, created_at, finished_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.UserID, r.BookID, r.Title, mustJSON(r.Authors), r.Format, r.SizeBytes,
		r.Status, r.R2Key, r.StorageKind, r.StorageBucket, r.Error, fmtTime(r.CreatedAt), fmtTime(r.FinishedAt))
	return err
}

// UpdateBookRecord 更新记录。
func (s *SQLiteStore) UpdateBookRecord(r *model.BookRecord) error {
	res, err := s.db.Exec(`UPDATE book_records SET title=?, authors=?, format=?, size_bytes=?,
		status=?, r2_key=?, storage_kind=?, storage_bucket=?, error=?, created_at=?, finished_at=? WHERE id=? AND user_id=?`,
		r.Title, mustJSON(r.Authors), r.Format, r.SizeBytes, r.Status, r.R2Key, r.StorageKind, r.StorageBucket, r.Error,
		fmtTime(r.CreatedAt), fmtTime(r.FinishedAt), r.ID, r.UserID)
	if err != nil {
		return err
	}
	return rowsAffected(res)
}

// GetBookRecord 查询记录（校验归属）。
func (s *SQLiteStore) GetBookRecord(userID, id string) (*model.BookRecord, error) {
	row := s.db.QueryRow(`SELECT id, user_id, book_id, title, authors, format, size_bytes, status,
		r2_key, storage_kind, storage_bucket, error, created_at, finished_at FROM book_records WHERE id=? AND user_id=?`, id, userID)
	var r model.BookRecord
	var authors, created, finished string
	err := row.Scan(&r.ID, &r.UserID, &r.BookID, &r.Title, &authors, &r.Format, &r.SizeBytes,
		&r.Status, &r.R2Key, &r.StorageKind, &r.StorageBucket, &r.Error, &created, &finished)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.Authors = jsonStrings(authors)
	r.CreatedAt = parseTime(created)
	r.FinishedAt = parseTime(finished)
	return &r, nil
}

// ListBookRecords 分页列出记录。
func (s *SQLiteStore) ListBookRecords(userID string, page, pageSize int) ([]*model.BookRecord, int, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM book_records WHERE user_id=?`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(`SELECT id, user_id, book_id, title, authors, format, size_bytes, status,
		r2_key, storage_kind, storage_bucket, error, created_at, finished_at FROM book_records WHERE user_id=?
		ORDER BY created_at DESC LIMIT ? OFFSET ?`, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := make([]*model.BookRecord, 0)
	for rows.Next() {
		var r model.BookRecord
		var authors, created, finished string
		if err := rows.Scan(&r.ID, &r.UserID, &r.BookID, &r.Title, &authors, &r.Format, &r.SizeBytes,
			&r.Status, &r.R2Key, &r.StorageKind, &r.StorageBucket, &r.Error, &created, &finished); err != nil {
			return nil, 0, err
		}
		r.Authors = jsonStrings(authors)
		r.CreatedAt = parseTime(created)
		r.FinishedAt = parseTime(finished)
		list = append(list, &r)
	}
	return list, total, rows.Err()
}

// DeleteBookRecord 删除记录。
func (s *SQLiteStore) DeleteBookRecord(userID, id string) error {
	res, err := s.db.Exec(`DELETE FROM book_records WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return err
	}
	return rowsAffected(res)
}

// ---- 统计 ----

// Stats 汇总用户统计。
func (s *SQLiteStore) Stats(userID string) model.Stats {
	var st model.Stats
	_ = s.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status='success' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status='success' THEN total_size ELSE 0 END),0) FROM tasks WHERE user_id=?`,
		userID).Scan(&st.TotalTasks, &st.SuccessTasks, &st.TotalSize)
	if st.TotalTasks > 0 {
		st.SuccessRate = float64(st.SuccessTasks) / float64(st.TotalTasks) * 100
	}
	return st
}

// rowsAffected 无行受影响视为 ErrNotFound。
func rowsAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// isUniqueErr 判定驱动返回的唯一约束冲突（驱动未导出哨兵类型，按文本判定）。
func isUniqueErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
