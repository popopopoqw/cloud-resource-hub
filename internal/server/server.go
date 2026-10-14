// Package server 实现 HTTP 服务：REST API + 内嵌前端静态资源。
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"resource-center/internal/book"
	"resource-center/internal/cfmon"
	"resource-center/internal/config"
	"resource-center/internal/mail"
	"resource-center/internal/model"
	"resource-center/internal/pan"
	"resource-center/internal/pan123web"
	"resource-center/internal/respool"
	"resource-center/internal/store"
)

// Server HTTP 服务。
type Server struct {
	servicesMu     sync.RWMutex
	cfg            *config.Config
	store          store.Store
	reg            *pan.Registry
	books          book.Provider
	pool           *respool.Fetcher
	panWeb         *pan123web.Client
	panFS          *panWebFS
	mailReg        *mail.Registry
	mon            cfmon.Monitor
	obj            ObjectStore
	mux            *http.ServeMux
	static         fs.FS
	forumHTTP      *http.Client // 测试可注入；生产为空时使用带超时的默认客户端
	startedAt      time.Time    // 进程启动时间，用于顶栏运行时长
	settingsCipher *credentialCipher
	settingsErr    error
	oauthMu        sync.Mutex
	gmailOAuth     map[string]gmailOAuthPending
	moduleHealthMu sync.Mutex
}

// New 创建服务并注册路由。static 为内嵌前端资源（根路径即 index.html）。
func New(cfg *config.Config, st store.Store, reg *pan.Registry, books book.Provider, mr *mail.Registry, mon cfmon.Monitor, pool *respool.Fetcher, pw *pan123web.Client, obj ObjectStore, static fs.FS) *Server {
	s := &Server{cfg: cfg, store: st, reg: reg, books: books, mailReg: mr, mon: mon, pool: pool, panWeb: pw, panFS: &panWebFS{c: pw}, obj: obj, mux: http.NewServeMux(), static: static, startedAt: time.Now(), gmailOAuth: map[string]gmailOAuthPending{}}
	s.settingsCipher, s.settingsErr = newCredentialCipher(cfg.DataDir)
	if s.settingsErr != nil {
		log.Printf("初始化配置加密失败: %v", s.settingsErr)
	} else {
		s.loadPersistedIntegrationSettings()
		s.applyAllIntegrationRuntimes()
	}
	if db, ok := st.(*store.SQLiteStore); ok {
		if err := db.BackfillLegacyBookRecords(cfg.R2Bucket, cfg.DataDir); err != nil {
			log.Printf("补齐历史书籍存储位置失败: %v", err)
		}
	}
	s.routes()
	return s
}

// Handler 返回根 Handler。withCommon（日志/恢复）→ withAuth（登录墙）→ 路由。
func (s *Server) Handler() http.Handler {
	return s.withCommon(s.withAuth(s.mux))
}

func (s *Server) routes() {
	m := s.mux

	// 认证（仅登录开放；站点不开放注册，账号由配置决定）
	m.HandleFunc("GET /login", s.handleLoginPage)
	m.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	m.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	m.HandleFunc("GET /api/v1/auth/me", s.handleMe)

	// API（除 health 外均需登录，由 withAuth 拦截）
	m.HandleFunc("GET /api/v1/health", s.handleHealth)
	m.HandleFunc("GET /api/v1/module-health", s.handleModuleHealth)
	m.HandleFunc("GET /api/v1/config", s.handleConfig)
	m.HandleFunc("GET /api/v1/settings/integrations", s.handleIntegrationSettings)
	m.HandleFunc("POST /api/v1/settings/integrations/test", s.handleTestIntegrationSettings)
	m.HandleFunc("POST /api/v1/settings/integrations", s.handleSaveIntegrationSettings)
	m.HandleFunc("POST /api/v1/settings/r2/buckets", s.handleListR2Buckets)
	m.HandleFunc("POST /api/v1/settings/gmail/oauth/start", s.handleGmailOAuthStart)
	m.HandleFunc("GET /api/v1/settings/gmail/oauth/callback", s.handleGmailOAuthCallback)
	m.HandleFunc("GET /api/v1/stats", s.handleStats)
	m.HandleFunc("POST /api/v1/parse", s.handleParse)
	m.HandleFunc("POST /api/v1/transfer", s.handleTransfer)
	m.HandleFunc("GET /api/v1/tasks", s.handleListTasks)
	m.HandleFunc("GET /api/v1/tasks/{id}", s.handleGetTask)
	m.HandleFunc("DELETE /api/v1/tasks/{id}", s.handleDeleteTask)

	// 影视全链路（论坛搜索 → 解锁 → 网盘落盘）
	m.HandleFunc("GET /api/v1/media/search", s.handleMediaSearch)
	m.HandleFunc("POST /api/v1/media/transfer", s.handleMediaTransfer)

	// 论坛资源源（pan1.me / 123panfx 实时抓取）
	m.HandleFunc("GET /api/v1/respool/sources", s.handleResPoolSources)
	m.HandleFunc("GET /api/v1/respool/list", s.handleResPoolList)
	m.HandleFunc("GET /api/v1/respool/cover", s.handleResPoolCover)
	m.HandleFunc("POST /api/v1/respool/login", s.handleResPoolLogin)
	m.HandleFunc("GET /api/forum/proxy/{source}/{path...}", s.handleForumProxyAll)
	m.HandleFunc("POST /api/forum/proxy/{source}/{path...}", s.handleForumProxyAll)
	m.HandleFunc("GET /api/v1/respool/thread", s.handleResPoolThread)
	m.HandleFunc("POST /api/v1/share", s.handleCreateShare) // 占位：转存后生成新分享

	// Cloudflare 账户监控
	m.HandleFunc("GET /api/v1/cfmon/status", s.handleCfmonStatus)
	m.HandleFunc("GET /api/v1/cfmon/overview", s.handleCfmonOverview)

	// 邮箱（多供应商：gmail 已上线，outlook 规划中）
	m.HandleFunc("GET /api/v1/mail/providers", s.handleMailProviders)
	m.HandleFunc("GET /api/v1/mail/{prov}/status", s.handleMailStatus)
	m.HandleFunc("GET /api/v1/mail/{prov}/messages", s.handleMailList)
	m.HandleFunc("GET /api/v1/mail/{prov}/messages/{id}", s.handleMailGet)
	m.HandleFunc("GET /api/v1/mail/{prov}/attachment/{id}/{attID}", s.handleMailAttachment)

	// 我的网盘（文件浏览与管理）
	m.HandleFunc("GET /api/v1/drive/list", s.handleDriveList)
	m.HandleFunc("GET /api/v1/drive/tree", s.handleDriveTree)
	m.HandleFunc("GET /api/v1/drive/capacity", s.handleDriveCapacity)
	m.HandleFunc("GET /api/v1/drive/dirsize/{file_id}", s.handleDriveDirSize)
	m.HandleFunc("GET /api/v1/drive/download/{file_id}", s.handleDriveDownload)
	m.HandleFunc("POST /api/v1/drive/mkdir", s.handleDriveMkdir)
	m.HandleFunc("POST /api/v1/drive/rename", s.handleDriveRename)
	m.HandleFunc("POST /api/v1/drive/move", s.handleDriveMove)
	m.HandleFunc("POST /api/v1/drive/trash", s.handleDriveTrash)

	// 书库（Z-Library → Cloudflare R2）
	m.HandleFunc("GET /api/v1/library/status", s.handleLibStatus)
	m.HandleFunc("GET /api/v1/library/search", s.handleLibSearch)
	m.HandleFunc("GET /api/v1/library/books/{id}", s.handleLibDetail)
	m.HandleFunc("GET /api/v1/library/shelf", s.handleLibShelfList)
	m.HandleFunc("POST /api/v1/library/shelf", s.handleLibShelfSave)
	m.HandleFunc("DELETE /api/v1/library/shelf/{bookID}", s.handleLibShelfDelete)
	m.HandleFunc("POST /api/v1/library/transfer", s.handleLibTransfer)
	m.HandleFunc("GET /api/v1/library/history", s.handleLibHistory)
	m.HandleFunc("DELETE /api/v1/library/history/{id}", s.handleLibHistoryDelete)
	m.HandleFunc("GET /api/v1/library/download/{id}", s.handleLibDownload)

	// 前端静态资源（embed 到二进制，单文件部署）
	m.Handle("GET /", http.FileServerFS(s.static))
}

// withCommon 通用中间件：日志、panic 恢复、安全头。
func (s *Server) withCommon(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic %s %s: %v", r.Method, r.URL.Path, rec)
				writeErr(w, http.StatusInternalServerError, "internal error")
			}
		}()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// ---- JSON 工具 ----

type resp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func writeOK(w http.ResponseWriter, data any) {
	writeJSONBody(w, http.StatusOK, resp{Code: 0, Msg: "ok", Data: data})
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSONBody(w, status, resp{Code: 1, Msg: msg})
}

func writeJSONBody(w http.ResponseWriter, status int, body resp) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func decodeBody(r *http.Request, v any) error {
	dec := json.NewDecoder(io_LimitReader(r))
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("请求体格式错误: %w", err)
	}
	return nil
}

func newID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

// startTransferTask 创建任务后在后台 goroutine 执行转存并推进度。
func (s *Server) startTransferTask(t *model.Task, req transferReq) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		p, ok := s.reg.Get(t.Provider)
		if !ok {
			s.finishTask(t, nil, "不支持的网盘")
			return
		}

		t.Status = model.TaskStatusRunning
		_ = s.store.UpdateTask(t)

		result, err := p.Transfer(ctx, pan.TransferRequest{
			ShareURL:     req.ShareURL,
			Code:         req.Code,
			FileIDs:      req.FileIDs,
			TargetFolder: req.TargetFolder,
		}, func(pct int) {
			t.Progress = pct
			_ = s.store.UpdateTask(t)
		})
		s.finishTask(t, result, errText(err))
	}()
}

func (s *Server) finishTask(t *model.Task, result *pan.TransferResult, errMsg string) {
	switch {
	case errMsg != "":
		t.Status = model.TaskStatusFailed
		t.Error = errMsg
	case result != nil && result.Error != "":
		t.Status = model.TaskStatusPartial
		t.Error = result.Error
	default:
		t.Status = model.TaskStatusSuccess
		if result != nil {
			t.Transferred = result.Files
		}
	}
	t.Progress = 100
	t.FinishedAt = time.Now()
	_ = s.store.UpdateTask(t)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
