// Package config 加载环境变量配置。
package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// LoadDotEnv 读取工作目录下的 .env 文件（不存在则跳过）。
// 仅支持 KEY=VALUE 行（# 注释、空行忽略，值可加引号）。
// 优先级：环境变量 > .env——仅当环境变量不存在（LookupEnv 为 false）时
// 才采用 .env 的值；环境变量显式设为空串同样视为"存在"，不被覆盖。
func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
}

// Config 应用配置，全部可通过环境变量覆盖。
type Config struct {
	Addr    string // 监听地址，默认 :8080
	BaseURL string // 对外基础 URL（Cloudflare 代理后填公网域名）

	Pan123ClientID     string
	Pan123ClientSecret string
	Pan123APIBase      string // 123云盘开放平台 API 地址
	Pan123AccessToken  string // 直接注入 access_token（签名自动获取未实现）
	Pan123Token        string // 123云盘 token（可与账密并存；401 自动续期）
	Pan123Account      string // 可选账密：启用 token 全自动续期（推荐）
	Pan123Password     string
	ResPoolReplyText   string // 自动回帖文案

	// 站点唯一账号（不开放注册）：启动时确保存在；后期通过环境变量修改。
	AuthUser string
	AuthPass string

	// SQLite 数据库路径；设为 "memory" 使用纯内存（重启清空）。
	SQLitePath string

	// 书库（Z-Library）与 Cloudflare R2 转存配置
	ZLibBaseURL  string // 站点域名（跳转模式也用它构造搜索链接）
	ZLibEmail    string // direct 模式凭据
	ZLibPassword string
	ZLibMode     string // 书库模式：mock（默认）/ direct（服务端直连）/ jump（浏览器跳转）

	R2AccountID       string // 旧配置兼容：仅在 CF_ACCOUNT_ID 未配置时使用
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	R2PublicBase      string // 可选：R2 自定义域/公开域，配置后下载走直链

	// ResPool 论坛资源源（pan1.me 等）抓取
	ResPoolCookie   string // 手动注入 cookie（最高优先级，可选）
	ResPoolAccount  string // 论坛账号（自动登录）
	ResPoolPassword string
	ResPoolHeaded   bool // 自动登录时弹出浏览器窗口（人工过极验滑块）

	// DataDir 本地对象存储根目录（R2 未配置时的兜底存储）
	DataDir string

	// Gmail 邮箱（L1 只读）：凭据齐全时接真实 Gmail，未配置时接口报错
	GmailClientID     string
	GmailClientSecret string
	GmailRefreshToken string
	GmailEmail        string // 展示用（授权账号）

	// Cloudflare 账户级监控（Workers/Zones/R2 用量与站点访问）
	CFAPIToken  string
	CFAccountID string // Cloudflare 与 R2 共用的账号 ID
}

// CloudflareAccountID 统一返回 Cloudflare/R2 使用的账号 ID。
// 旧的 R2_ACCOUNT_ID 仅用于迁移期回退，不再作为独立 R2 配置。
func (c Config) CloudflareAccountID() string {
	if c.CFAccountID != "" {
		return c.CFAccountID
	}
	return c.R2AccountID
}

// Load 从环境变量读取配置（先加载工作目录 .env，真实环境变量优先）。
func Load() *Config {
	LoadDotEnv(".env")
	return &Config{
		Addr:               env("PORT", ":8080"),
		BaseURL:            env("BASE_URL", ""),
		Pan123ClientID:     env("PAN123_CLIENT_ID", ""),
		Pan123ClientSecret: env("PAN123_CLIENT_SECRET", ""),
		Pan123AccessToken:  env("PAN123_ACCESS_TOKEN", ""),
		Pan123Token:        env("PAN123_TOKEN", ""),
		Pan123Account:      env("PAN123_ACCOUNT", ""),
		Pan123Password:     env("PAN123_PASSWORD", ""),
		ResPoolReplyText:   env("RESPOOL_REPLY_TEXT", ""),
		ResPoolCookie:      env("RESPOOL_COOKIE", ""),
		ResPoolAccount:     env("RESPOOL_ACCOUNT", ""),
		ResPoolPassword:    env("RESPOOL_PASSWORD", ""),
		ResPoolHeaded:      envBool("RESPOOL_HEADED", false),
		Pan123APIBase:      env("PAN123_API_BASE", "https://open-api.123pan.com"),
		AuthUser:           env("AUTH_USER", "demo"),
		AuthPass:           env("AUTH_PASS", "demo123"),
		SQLitePath:         env("SQLITE_PATH", "./resource-center.db"),
		ZLibBaseURL:        env("ZLIB_BASE_URL", "https://z-lib.sk"),
		ZLibEmail:          env("ZLIB_EMAIL", ""),
		ZLibPassword:       env("ZLIB_PASSWORD", ""),
		ZLibMode:           env("ZLIB_MODE", ""),
		R2AccountID:        env("R2_ACCOUNT_ID", ""),
		R2AccessKeyID:      env("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey:  env("R2_SECRET_ACCESS_KEY", ""),
		R2Bucket:           env("R2_BUCKET", ""),
		R2PublicBase:       env("R2_PUBLIC_BASE", ""),
		DataDir:            env("DATA_DIR", "./data"),
		GmailClientID:      env("GMAIL_CLIENT_ID", ""),
		GmailClientSecret:  env("GMAIL_CLIENT_SECRET", ""),
		GmailRefreshToken:  env("GMAIL_REFRESH_TOKEN", ""),
		GmailEmail:         env("GMAIL_EMAIL", ""),
		CFAPIToken:         env("CF_API_TOKEN", ""),
		CFAccountID:        env("CF_ACCOUNT_ID", ""),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
