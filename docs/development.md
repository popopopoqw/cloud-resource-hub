# 开发与架构

## 技术栈

- Go 1.26+
- 原生 HTML、CSS、JavaScript
- `go:embed` 内嵌前端资源
- `modernc.org/sqlite`，无需 CGO
- chromedp 论坛会话、uTLS TLS 指纹兼容
- PBKDF2-SHA256 密码摘要与 HttpOnly Cookie 会话

## 项目结构

```text
resource-center/
├── .github/workflows/      GitHub Actions：CI 与 Release
├── cloudflare/             Tunnel 与 Worker 示例
├── deploy/                 本机、GCE、systemd 部署脚本
├── docs/                   配置、设计与部署文档
├── internal/
│   ├── book/               书库 Provider
│   ├── cfmon/              Cloudflare API 与统计
│   ├── config/             .env 与环境变量
│   ├── gmail/、mail/        Gmail 与通用邮箱接口
│   ├── pan/、pan123web/     123 云盘开放平台及内部 API
│   ├── respool/            论坛抓取、会话和资源解锁
│   ├── server/             HTTP 路由与业务处理
│   └── store/              SQLite 与内存存储
├── web/                    内嵌的前端单页应用
├── Dockerfile
├── go.mod
└── main.go
```

## 模块健康检测

服务启动时立即检测一次功能模块依赖，之后每 30 分钟执行一次。结果和检测时间写入 SQLite 的 `module_health` 表，概览页的 `/api/v1/module-health` 只读取缓存，不会因为用户反复进入页面而触发外部探测。

单轮检测最多等待 8 秒，各服务并发探测。尚未生成缓存时页面显示“待检测”。

## 本地验证

```bash
gofmt -w .
go vet ./...
go test ./...
go build -o resource-center .
node --check web/static/app.js
```

GitHub Actions 和版本发布细节见 [GitHub Actions 文档](github-actions.md)。

## 扩展入口

- 新增网盘：实现 `internal/pan` 中的 Provider 或 FS 接口，并在 `main.go` 注册
- 新增邮箱：实现 `internal/mail.Provider` 并加入 Registry
- 新增概览模块：在 `web/static/app.js` 的 `MODULES` 中登记
- 新增持久化后端：实现 `internal/store.Store`

## 使用声明

本项目仅供学习、研究与个人娱乐使用，严禁用于任何商业用途。项目不存储、复制或传播任何网盘文件，仅提供链接解析与转存工具。使用者应确保资源来源及使用方式合法合规，并自行承担因下载、保存、分享或商业使用产生的全部风险与责任；项目作者及维护者不对由此造成的任何直接或间接损失承担责任。
