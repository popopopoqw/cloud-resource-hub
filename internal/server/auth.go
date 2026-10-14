package server

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"resource-center/internal/model"
	"resource-center/internal/store"
)

const (
	sessionCookie = "pt_session"
	sessionTTL    = 7 * 24 * time.Hour
	pbkdf2Iters   = 120_000
)

type ctxKey int

const ctxUserID ctxKey = 1

// currentUser 从请求上下文取登录用户 ID（由 withAuth 注入）。
func currentUser(r *http.Request) string {
	uid, _ := r.Context().Value(ctxUserID).(string)
	return uid
}

// hashPassword PBKDF2-SHA256 摘要，返回 (hash, salt) 的 hex。
func hashPassword(password, saltHex string) string {
	salt, _ := hex.DecodeString(saltHex)
	key, _ := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iters, 32)
	return hex.EncodeToString(key)
}

func newSalt() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func validUsername(name string) bool {
	if len(name) < 2 || len(name) > 32 {
		return false
	}
	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			c == '_', c == '-', c >= 0x4e00 && c <= 0x9fa5: // 汉字
		default:
			return false
		}
	}
	return true
}

// ---- 认证 API ----

type credReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleLogin 登录。站点不开放注册，账号由服务端配置（见 EnsureSeedUser）。
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req credReq
	if err := decodeBody(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	u, err := s.store.UserByUsername(strings.TrimSpace(req.Username))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	got := hashPassword(req.Password, u.Salt)
	if subtle.ConstantTimeCompare([]byte(got), []byte(u.PassHash)) != 1 {
		writeErr(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	s.openSession(w, u.ID)
	writeOK(w, map[string]any{"id": u.ID, "username": u.Username})
}

// handleLogout 退出登录。
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		_ = s.store.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	writeOK(w, nil)
}

// handleMe 当前登录用户信息。
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.UserByID(currentUser(r))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "未登录")
		return
	}
	writeOK(w, map[string]any{"id": u.ID, "username": u.Username, "created_at": u.CreatedAt})
}

// EnsureSeedUser 启动时确保配置的账号存在（AUTH_USER/AUTH_PASS）。
// 用户名或密码为空则跳过；用户已存在则不覆盖（内存存储下重启即重建）。
// 站点不开放注册，唯一账号由此配置决定。
func EnsureSeedUser(st store.Store, username, password string) error {
	if username == "" || password == "" {
		return nil
	}
	if !validUsername(username) {
		return errors.New("AUTH_USER 配置非法：需 2-32 位，仅限中英文、数字、下划线、短横线")
	}
	if len(password) < 6 || len(password) > 64 {
		return errors.New("AUTH_PASS 配置非法：需 6-64 位")
	}
	if _, err := st.UserByUsername(username); err == nil {
		return nil // 已存在
	}
	salt := newSalt()
	u := &model.User{
		// ID 基于用户名生成（用户名已通过合法性校验），换账号名不冲突
		ID:        "user_" + username,
		Username:  username,
		Salt:      salt,
		PassHash:  hashPassword(password, salt),
		CreatedAt: time.Now(),
	}
	if err := st.CreateUser(u); err != nil && !errors.Is(err, store.ErrDupUser) {
		return err
	}
	return nil
}

// openSession 创建会话并下发 cookie。
func (s *Server) openSession(w http.ResponseWriter, userID string) {
	sess := &model.Session{
		Token:     newToken(),
		UserID:    userID,
		ExpiresAt: time.Now().Add(sessionTTL),
	}
	_ = s.store.SaveSession(sess)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sess.Token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(s.cfg.BaseURL, "https://"),
	})
}

// ---- 登录墙中间件 ----

// publicPath 未登录可访问的路径。
func publicPath(path, method string) bool {
	switch {
	case path == "/login",
		path == "/favicon.ico",
		path == "/robots.txt",
		strings.HasPrefix(path, "/static/"),
		path == "/api/v1/health",
		path == "/api/v1/auth/login" && method == http.MethodPost:
		return true
	}
	return false
}

// withAuth 登录墙：未登录时页面 302 /login、API 返回 401；
// 已登录时把 userID 注入请求上下文。
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if publicPath(r.URL.Path, r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		uid := s.sessionUser(r)
		if uid == "" {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeErr(w, http.StatusUnauthorized, "未登录或会话已过期")
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// sessionUser 校验会话有效性，返回用户 ID；会话临近过期时滑动续期。
func (s *Server) sessionUser(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return ""
	}
	sess, err := s.store.SessionByToken(c.Value)
	if err != nil || time.Now().After(sess.ExpiresAt) {
		_ = s.store.DeleteSession(c.Value)
		return ""
	}
	// 剩余不足一半时滑动续期（仅延后服务端过期时间，cookie 本身 7 天有效）
	if time.Until(sess.ExpiresAt) < sessionTTL/2 {
		sess.ExpiresAt = time.Now().Add(sessionTTL)
		_ = s.store.SaveSession(sess)
	}
	return sess.UserID
}

// handleLoginPage 登录页；已登录访问则跳回首页。
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if s.sessionUser(r) != "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	b, err := fs.ReadFile(s.static, "login.html")
	if err != nil {
		http.Error(w, "login page missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}
