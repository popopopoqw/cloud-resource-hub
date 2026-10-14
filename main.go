// resource-center 资源中心 Web 应用。
//
// 用法:
//
// 账号凭据经设置中心（网页内）配置，或以环境变量注入：
//
//	PAN123_TOKEN=xxx ./resource-center    # 123云盘（内部 API 通道）
//	无凭据启动时各模块如实报「未配置」，不提供演示数据
package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"resource-center/internal/book"
	"resource-center/internal/cfmon"
	"resource-center/internal/config"
	"resource-center/internal/gmail"
	"resource-center/internal/mail"
	"resource-center/internal/pan"
	"resource-center/internal/pan123web"
	"resource-center/internal/r2"
	"resource-center/internal/respool"
	"resource-center/internal/server"
	"resource-center/internal/store"
	"resource-center/internal/zlib"
)

//go:embed web
var webFS embed.FS

func main() {
	// Gmail OAuth 辅助授权：--gmail-auth 换取 refresh token 后退出
	if len(os.Args) > 1 && os.Args[1] == "--gmail-auth" {
		runGmailAuth()
		return
	}

	// 123云盘 token 换取：--pan123-token <账号> <密码>（账密仅在参数中瞬时使用，不落盘）
	if len(os.Args) > 1 && os.Args[1] == "--pan123-token" {
		runPan123Token(os.Args[2:])
		return
	}

	cfg := config.Load()

	st, closeStore := openStore(cfg)
	defer closeStore()
	if err := server.EnsureSeedUser(st, cfg.AuthUser, cfg.AuthPass); err != nil {
		log.Printf("站点账号初始化失败: %v", err)
	}
	reg := buildProviders(cfg)
	books := buildBookProvider(cfg)
	obj := buildObjectStore(cfg)
	mr := buildMailRegistry(cfg)
	mon := buildMonitor(cfg)
	respool.SetReplyText(cfg.ResPoolReplyText)
	pool := respool.New(cfg.ResPoolCookie, cfg.ResPoolAccount, cfg.ResPoolPassword, cfg.DataDir, cfg.ResPoolHeaded)
	log.Printf("[debug] ResPoolHeaded=%v Account=%v DataDir=%s", cfg.ResPoolHeaded, cfg.ResPoolAccount != "", cfg.DataDir)
	panWeb := pan123web.New(cfg.Pan123Token, cfg.Pan123Account, cfg.Pan123Password)
	switch {
	case panWeb.Configured() && cfg.Pan123Account != "":
		log.Println("123云盘（内部API）：全自动模式（token 失效自动续期，无需人工）")
	case panWeb.Configured():
		log.Println("123云盘（内部API）：token 模式（未配账密，失效时需手动换新；建议加 PAN123_ACCOUNT/PASSWORD 启用自动续期）")
	default:
		log.Println("123云盘（内部API）：未配置凭据（PAN123_ACCOUNT/PAN123_PASSWORD 全自动，或 PAN123_TOKEN）")
	}
	if pool.HasCookie() {
		log.Println("资源源：登录态就绪（cookie 注入或持久化会话）")
	} else if cfg.ResPoolAccount != "" {
		log.Println("资源源：尚未登录（请在资源广场点击「连接论坛」完成验证）")
	} else {
		log.Println("资源源：无登录态（列表可抓，正文提取需在资源广场连接论坛）")
	}

	srv := server.New(cfg, st, reg, books, mr, mon, pool, panWeb, obj, subFS(webFS))
	backgroundCtx, cancelBackground := context.WithCancel(context.Background())
	defer cancelBackground()
	srv.StartBackgroundTasks(backgroundCtx)
	addr := cfg.Addr
	log.Printf("resource-center 启动中 addr=%s db=%s", addr, cfg.SQLitePath)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}

// runPan123Token 一次性用账密换 token（账密仅作为命令行参数瞬时使用，不落盘）：
// ./resource-center --pan123-token <账号> <密码>
func runPan123Token(args []string) {
	if len(args) < 2 {
		fmt.Println("用法: ./resource-center --pan123-token <手机号或邮箱> <密码>")
		fmt.Println("（账密仅本次换取使用，不会写入任何文件）")
		os.Exit(1)
	}
	token, err := pan123web.ExchangeToken(args[0], args[1])
	if err != nil {
		log.Fatalf("换取失败: %v", err)
	}
	fmt.Println()
	fmt.Println("════════ 123云盘 token 换取成功 ════════")
	fmt.Println("填入 .env 后重启（账密未留存，仅 token 落盘）：")
	fmt.Println()
	fmt.Printf("PAN123_TOKEN=%s\n", token)
	fmt.Println()
	fmt.Println("注意：token 失效（页面提示 401）时重新执行本命令换新。")
}

// runGmailAuth CLI 辅助授权：GMAIL_CLIENT_ID/GMAIL_CLIENT_SECRET 环境变量
// 必须已配置，执行后打印 refresh token。
func runGmailAuth() {
	cfg := config.Load()
	if cfg.GmailClientID == "" || cfg.GmailClientSecret == "" {
		log.Fatal("请先设置 GMAIL_CLIENT_ID 和 GMAIL_CLIENT_SECRET（Google Cloud OAuth 客户端）")
	}
	token, err := gmail.RunAuthCLI(cfg.GmailClientID, cfg.GmailClientSecret)
	if err != nil {
		log.Fatalf("授权失败: %v", err)
	}
	fmt.Println()
	fmt.Println("════════ 授权成功 ════════")
	fmt.Println("请将以下环境变量写入部署配置（然后重启服务）：")
	fmt.Println()
	fmt.Printf("  GMAIL_REFRESH_TOKEN=%s\n", token)
	fmt.Printf("  GMAIL_EMAIL=<你的邮箱地址>\n")
	fmt.Println()
	fmt.Println("注意：refresh token 仅本次显示，请妥善保存。")
}

// openStore 按配置选择存储：默认 SQLite 持久化；SQLITE_PATH=memory 时纯内存。
func openStore(cfg *config.Config) (store.Store, func()) {
	if cfg.SQLitePath == "" || cfg.SQLitePath == "memory" {
		log.Println("存储：内存模式（重启后数据清空）")
		return store.NewMemStore(), func() {}
	}
	db, err := store.NewSQLiteStore(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("打开 SQLite 失败(%s): %v", cfg.SQLitePath, err)
	}
	log.Printf("存储：SQLite(%s)", cfg.SQLitePath)
	return db, func() { _ = db.Close() }
}

// buildMonitor Cloudflare 监控：配置 CF_API_TOKEN + Account ID 时接真实账户，
// 未配置返回 nil（接口层报「未配置」，不提供演示数据）。
func buildMonitor(cfg *config.Config) cfmon.Monitor {
	acct := cfg.CloudflareAccountID()
	if cfg.CFAPIToken != "" && acct != "" {
		log.Printf("Cloudflare 监控：真实账户（%s）", acct)
		return cfmon.NewClient(cfg.CFAPIToken, acct)
	}
	log.Println("Cloudflare 监控：未配置（CF_API_TOKEN/CF_ACCOUNT_ID），请到设置中心配置")
	return nil
}

// buildMailRegistry 邮箱供应商注册表。Gmail 已上线；Outlook 等
// 后期按 mail.Provider 接口实现后在此注册即可（前端自动出现）。
// 凭据不齐全时保持空注册表（接口层报「未配置」，不提供演示数据）。
func buildMailRegistry(cfg *config.Config) *mail.Registry {
	mr := mail.NewRegistry()
	if cfg.GmailClientID != "" && cfg.GmailClientSecret != "" && cfg.GmailRefreshToken != "" {
		email := cfg.GmailEmail
		if email == "" {
			email = "已授权账号"
		}
		log.Printf("邮箱：Gmail（%s）", email)
		mr.Register(gmail.NewClient(cfg.GmailClientID, cfg.GmailClientSecret, cfg.GmailRefreshToken, cfg.GmailEmail))
	} else {
		log.Println("邮箱：未配置（GMAIL_CLIENT_ID/SECRET/REFRESH_TOKEN），请到设置中心配置")
	}
	return mr
}

// buildObjectStore 对象存储：R2 凭据齐全时用 Cloudflare R2，
// 否则兜底本地目录（DATA_DIR）。
func buildObjectStore(cfg *config.Config) server.ObjectStore {
	// R2 与监控使用同一个 Cloudflare Account ID。
	r2acct := cfg.CloudflareAccountID()
	r2c := r2.New(r2acct, cfg.R2AccessKeyID, cfg.R2SecretAccessKey, cfg.R2Bucket)
	if r2c.Configured() {
		log.Printf("对象存储：Cloudflare R2（bucket=%s）", cfg.R2Bucket)
		return r2Store{r2c}
	}
	log.Printf("对象存储：本地目录（%s）——R2 未配置，配置 R2_* 环境变量后自动切换", cfg.DataDir)
	return server.NewLocalStore(cfg.DataDir)
}

// r2Store 适配器：*r2.Client 补上 Kind()。
type r2Store struct{ c *r2.Client }

func (s r2Store) Kind() string { return "r2" }
func (s r2Store) PutObject(key string, r io.Reader, ct string, size int64) (int64, error) {
	return s.c.PutObject(key, r, ct, size)
}
func (s r2Store) GetObject(key string) (io.ReadCloser, int64, string, error) {
	return s.c.GetObject(key)
}

// buildBookProvider 书库模式（ZLIB_MODE）：
//   - jump  浏览器跳转（默认）：站内入口 + 新窗口打开 Z-Library 搜索
//   - direct 服务端直连（Z-Library DiamWall 反爬当前封锁此路径，留作后续解封复用）
func buildBookProvider(cfg *config.Config) book.Provider {
	mode := cfg.ZLibMode
	if mode == "" {
		mode = "jump" // 无演示降级：默认浏览器跳转（真实入口）
	}
	switch mode {
	case "jump":
		log.Printf("书库：浏览器跳转模式（%s）", cfg.ZLibBaseURL)
		return zlib.NewJump(cfg.ZLibBaseURL)
	default:
		log.Printf("书库：Z-Library 直连模式（%s）——注意 DiamWall 反爬可能拦截", maskEmail(cfg.ZLibEmail))
		return zlib.New(zlib.Options{
			BaseURL:  cfg.ZLibBaseURL,
			Email:    cfg.ZLibEmail,
			Password: cfg.ZLibPassword,
		})
	}
}

// buildProviders 根据配置装配网盘 Provider（转存工作台用的开放平台通道）。
// 仅在配置 PAN123_ACCESS_TOKEN 时注册真实 123pan 客户端；
// 未配置时保持空注册表（接口层报「未配置」，不提供演示数据）。
// 注：资源广场/影视转存走 pan123web 内部 API（PAN123_TOKEN），与此通道独立。
func buildProviders(cfg *config.Config) *pan.Registry {
	reg := pan.NewRegistry()
	if cfg.Pan123AccessToken != "" {
		log.Println("网盘（开放平台通道）：123云盘真实对接（PAN123_ACCESS_TOKEN）")
		reg.Register(pan.NewPan123(cfg.Pan123APIBase, cfg.Pan123ClientID, cfg.Pan123ClientSecret))
		return reg
	}
	log.Println("网盘（开放平台通道）：未配置（PAN123_ACCESS_TOKEN）；影视转存走内部 API 通道不受影响")
	return reg
}

// maskEmail 日志脱敏：yu***@outlook.com
func maskEmail(e string) string {
	if at := strings.Index(e, "@"); at > 1 {
		return e[:2] + "***" + e[at:]
	}
	return e
}

func subFS(root embed.FS) fs.FS {
	sub, err := fs.Sub(root, "web")
	if err != nil {
		log.Fatal(err)
	}
	return sub
}
