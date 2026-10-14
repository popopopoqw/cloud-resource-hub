// Package store 提供数据存储抽象。当前为内存实现；
// 对接 Cloudflare 时可实现同一接口改用 D1 / KV / R2，上层代码无需改动。
package store

import (
	"sort"
	"sync"

	"resource-center/internal/model"
)

// Store 存储接口。除用户/会话外，其余数据均按 userID 隔离。
type Store interface {
	// 用户与会话
	CreateUser(u *model.User) error
	UserByUsername(username string) (*model.User, error)
	UserByID(id string) (*model.User, error)
	SaveSession(s *model.Session) error
	SessionByToken(token string) (*model.Session, error)
	DeleteSession(token string) error

	// 任务（按用户）
	CreateTask(t *model.Task) error
	UpdateTask(t *model.Task) error
	GetTask(userID, id string) (*model.Task, error)
	ListTasks(userID, status string, page, pageSize int) ([]*model.Task, int, error)
	DeleteTask(userID, id string) error

	// 账号（按用户）
	SaveAccount(a *model.Account) error
	ListAccounts(userID string) ([]*model.Account, error)
	DeleteAccount(userID, id string) error

	// 外部服务配置（单实例全局配置；敏感值由服务层加密后传入）
	SaveIntegrationSettings(section string, items []model.IntegrationSetting) error
	ListIntegrationSettings() ([]model.IntegrationSetting, error)

	// 功能模块健康状态（全局缓存）
	SaveModuleHealth(items []model.ModuleHealth) error
	ListModuleHealth() ([]model.ModuleHealth, error)

	// 书架收藏（按用户；同一本书重复收藏幂等）
	SaveShelfItem(item *model.ShelfItem) error
	ListShelf(userID string) ([]*model.ShelfItem, error)
	DeleteShelfItem(userID, bookID string) error

	// 书籍转存记录（按用户）
	CreateBookRecord(r *model.BookRecord) error
	UpdateBookRecord(r *model.BookRecord) error
	GetBookRecord(userID, id string) (*model.BookRecord, error)
	ListBookRecords(userID string, page, pageSize int) ([]*model.BookRecord, int, error)
	DeleteBookRecord(userID, id string) error

	// 统计（按用户）
	Stats(userID string) model.Stats
}

// MemStore 内存实现，带读写锁。适用于单实例部署与开发联调。
type MemStore struct {
	mu           sync.RWMutex
	users        map[string]*model.User    // key: username
	sessions     map[string]*model.Session // key: token
	tasks        map[string]*model.Task
	taskIDs      []string // 保持插入顺序
	accounts     map[string]*model.Account
	settings     map[string]model.IntegrationSetting // key: section/key
	moduleHealth map[string]model.ModuleHealth       // key: module id
	shelf        map[string]*model.ShelfItem         // key: id
	bookRecs     map[string]*model.BookRecord        // key: id
	bookRecIDs   []string
}

// NewMemStore 创建内存存储。
func NewMemStore() *MemStore {
	return &MemStore{
		users:        map[string]*model.User{},
		sessions:     map[string]*model.Session{},
		tasks:        map[string]*model.Task{},
		accounts:     map[string]*model.Account{},
		settings:     map[string]model.IntegrationSetting{},
		moduleHealth: map[string]model.ModuleHealth{},
		shelf:        map[string]*model.ShelfItem{},
		bookRecs:     map[string]*model.BookRecord{},
	}
}

// SaveModuleHealth 覆盖模块最近一次探测结果。
func (s *MemStore) SaveModuleHealth(items []model.ModuleHealth) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range items {
		s.moduleHealth[item.ModuleID] = item
	}
	return nil
}

// ListModuleHealth 返回全部模块最近一次探测结果。
func (s *MemStore) ListModuleHealth() ([]model.ModuleHealth, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.ModuleHealth, 0, len(s.moduleHealth))
	for _, item := range s.moduleHealth {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModuleID < out[j].ModuleID })
	return out, nil
}

// SaveIntegrationSettings 原子覆盖指定分组中传入的键。
func (s *MemStore) SaveIntegrationSettings(_ string, items []model.IntegrationSetting) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, item := range items {
		s.settings[item.Section+"/"+item.Key] = item
	}
	return nil
}

// ListIntegrationSettings 返回全部外部服务配置。
func (s *MemStore) ListIntegrationSettings() ([]model.IntegrationSetting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.IntegrationSetting, 0, len(s.settings))
	for _, item := range s.settings {
		out = append(out, item)
	}
	return out, nil
}

// ---- 用户与会话 ----

// CreateUser 新增用户，用户名唯一。
func (s *MemStore) CreateUser(u *model.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[u.Username]; ok {
		return ErrDupUser
	}
	s.users[u.Username] = u
	return nil
}

// UserByUsername 按用户名查询。
func (s *MemStore) UserByUsername(username string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *u
	return &cp, nil
}

// UserByID 按 ID 查询。
func (s *MemStore) UserByID(id string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.ID == id {
			cp := *u
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

// SaveSession 保存会话。
func (s *MemStore) SaveSession(sess *model.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.Token] = sess
	return nil
}

// SessionByToken 查询会话。
func (s *MemStore) SessionByToken(token string) (*model.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[token]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *sess
	return &cp, nil
}

// DeleteSession 注销会话。
func (s *MemStore) DeleteSession(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
	return nil
}

// ---- 任务 ----

// CreateTask 新增任务。
func (s *MemStore) CreateTask(t *model.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[t.ID] = t
	s.taskIDs = append(s.taskIDs, t.ID)
	return nil
}

// UpdateTask 更新任务（校验归属）。
func (s *MemStore) UpdateTask(t *model.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.tasks[t.ID]
	if !ok || old.UserID != t.UserID {
		return ErrNotFound
	}
	s.tasks[t.ID] = t
	return nil
}

// GetTask 按 ID 查询任务（校验归属，不匹配视为不存在）。
func (s *MemStore) GetTask(userID, id string) (*model.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	if !ok || t.UserID != userID {
		return nil, ErrNotFound
	}
	cp := *t
	return &cp, nil
}

// ListTasks 分页列出指定用户的任务，按创建时间倒序。status 为空时不过滤。
func (s *MemStore) ListTasks(userID, status string, page, pageSize int) ([]*model.Task, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*model.Task
	for i := len(s.taskIDs) - 1; i >= 0; i-- {
		t := s.tasks[s.taskIDs[i]]
		if t == nil || t.UserID != userID {
			continue
		}
		if status != "" && t.Status != status {
			continue
		}
		cp := *t
		list = append(list, &cp)
	}

	total := len(list)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return list[start:end], total, nil
}

// DeleteTask 删除任务（校验归属）。
func (s *MemStore) DeleteTask(userID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok || t.UserID != userID {
		return ErrNotFound
	}
	delete(s.tasks, id)
	for i, v := range s.taskIDs {
		if v == id {
			s.taskIDs = append(s.taskIDs[:i], s.taskIDs[i+1:]...)
			break
		}
	}
	return nil
}

// ---- 账号 ----

// SaveAccount 保存账号（新增或覆盖）。
func (s *MemStore) SaveAccount(a *model.Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.accounts[a.ID]; ok && old.UserID != a.UserID {
		return ErrNotFound
	}
	s.accounts[a.ID] = a
	return nil
}

// ListAccounts 列出指定用户的账号。
func (s *MemStore) ListAccounts(userID string) ([]*model.Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Account, 0)
	for _, a := range s.accounts {
		if a.UserID != userID {
			continue
		}
		cp := *a
		out = append(out, &cp)
	}
	return out, nil
}

// DeleteAccount 删除账号（校验归属）。
func (s *MemStore) DeleteAccount(userID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[id]
	if !ok || a.UserID != userID {
		return ErrNotFound
	}
	delete(s.accounts, id)
	return nil
}

// Stats 汇总指定用户的统计。
func (s *MemStore) Stats(userID string) model.Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := model.Stats{}
	for _, t := range s.tasks {
		if t.UserID != userID {
			continue
		}
		st.TotalTasks++
		if t.Status == model.TaskStatusSuccess {
			st.SuccessTasks++
			st.TotalSize += t.TotalSize
		}
	}
	if st.TotalTasks > 0 {
		st.SuccessRate = float64(st.SuccessTasks) / float64(st.TotalTasks) * 100
	}
	return st
}

// ---- 书架与书籍转存记录 ----

// SaveShelfItem 收藏书籍（同书重复收藏幂等：覆盖旧记录）。
func (s *MemStore) SaveShelfItem(item *model.ShelfItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// 同一用户同一本书只保留一条
	for id, old := range s.shelf {
		if old.UserID == item.UserID && old.BookID == item.BookID {
			item.ID = old.ID
			delete(s.shelf, id)
			break
		}
	}
	if item.ID == "" {
		item.ID = "shelf_" + item.BookID + "_" + item.UserID
	}
	s.shelf[item.ID] = item
	return nil
}

// ListShelf 列出书架（按收藏时间倒序）。
func (s *MemStore) ListShelf(userID string) ([]*model.ShelfItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.ShelfItem, 0)
	for _, it := range s.shelf {
		if it.UserID != userID {
			continue
		}
		cp := *it
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AddedAt.After(out[j].AddedAt) })
	return out, nil
}

// DeleteShelfItem 取消收藏（按 bookID）。
func (s *MemStore) DeleteShelfItem(userID, bookID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, it := range s.shelf {
		if it.UserID == userID && it.BookID == bookID {
			delete(s.shelf, id)
			return nil
		}
	}
	return ErrNotFound
}

// CreateBookRecord 新增书籍转存记录。
func (s *MemStore) CreateBookRecord(r *model.BookRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bookRecs[r.ID] = r
	s.bookRecIDs = append(s.bookRecIDs, r.ID)
	return nil
}

// UpdateBookRecord 更新记录（校验归属）。
func (s *MemStore) UpdateBookRecord(r *model.BookRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.bookRecs[r.ID]
	if !ok || old.UserID != r.UserID {
		return ErrNotFound
	}
	s.bookRecs[r.ID] = r
	return nil
}

// GetBookRecord 查询记录（校验归属）。
func (s *MemStore) GetBookRecord(userID, id string) (*model.BookRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.bookRecs[id]
	if !ok || r.UserID != userID {
		return nil, ErrNotFound
	}
	cp := *r
	return &cp, nil
}

// ListBookRecords 分页列出记录（时间倒序）。
func (s *MemStore) ListBookRecords(userID string, page, pageSize int) ([]*model.BookRecord, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*model.BookRecord
	for i := len(s.bookRecIDs) - 1; i >= 0; i-- {
		r := s.bookRecs[s.bookRecIDs[i]]
		if r == nil || r.UserID != userID {
			continue
		}
		cp := *r
		list = append(list, &cp)
	}
	total := len(list)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return list[start:end], total, nil
}

// DeleteBookRecord 删除记录（校验归属）。
func (s *MemStore) DeleteBookRecord(userID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.bookRecs[id]
	if !ok || r.UserID != userID {
		return ErrNotFound
	}
	delete(s.bookRecs, id)
	for i, v := range s.bookRecIDs {
		if v == id {
			s.bookRecIDs = append(s.bookRecIDs[:i], s.bookRecIDs[i+1:]...)
			break
		}
	}
	return nil
}
