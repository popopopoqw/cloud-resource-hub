# Gmail 邮箱配置指南（resource-center）

> 邮箱模块为 L1 只读（scope：`gmail.readonly`）。推荐直接在账号设置页完成授权：
> **① Google Cloud 创建 OAuth 凭据**（账号所有者操作）→
> **② 设置页点击「授权 Gmail」** → 服务端自动加密保存并立即生效。

## ① 创建 OAuth 凭据（约 5 分钟）

1. 打开 [console.cloud.google.com](https://console.cloud.google.com/)，
   新建项目（或选现有项目），如 `resource-center`
2. **API 和服务 → 启用 API 和服务** → 搜索 **Gmail API** → 启用
3. **OAuth 同意屏**：User Type 选 **External** → 填应用名与支持邮箱 → 保存；
   最后在 **测试用户** 中添加你的 Gmail 地址（⚠️ 不加会导致授权时 403）
4. **凭据 → 创建凭据 → OAuth 客户端 ID** → 应用类型选 **桌面应用** → 创建
5. 复制 **客户端 ID**（`xxx.apps.googleusercontent.com`）与 **客户端密钥**

## ② 设置页授权（推荐）

1. 打开 `#/settings`，在「Gmail 邮箱」填写邮箱、Client ID 和 Client Secret
2. 点击「授权 Gmail」，在 Google 弹窗中登录并同意只读权限
3. 回调后服务端自动换取 Refresh Token、调用 Gmail Profile 测试连接
4. 测试通过后配置写入 SQLite 并立即热更新，无需重启

Client Secret 与 Refresh Token 使用 AES-256-GCM 加密后入库，页面只显示 12 位掩码；
Refresh Token 不会返回浏览器。邮箱地址属于普通标识，按明文保存和展示。

> 公网部署时请正确配置 `BASE_URL=https://你的域名`，以便生成 OAuth 回调地址。

## ③ CLI 授权（备用）

把凭据写入 `.env`（裸值，勿加引号）：

```ini
GMAIL_CLIENT_ID=<客户端ID>
GMAIL_CLIENT_SECRET=<客户端密钥>
```

执行授权（会在本机 127.0.0.1:8484 起回调监听，端口占用时用 `GMAIL_AUTH_PORT` 改）：

```bash
cd /opt/WorkSpace/todo/resource-center
./resource-center --gmail-auth
```

- 国内网络需代理出站（授权 URL 的交换请求要访问 googleapis.com）：

  ```bash
  HTTPS_PROXY=http://127.0.0.1:7897 ./resource-center --gmail-auth
  ```

- 终端打印授权链接 → 浏览器打开 → 登录 Google → 同意（只读权限）
- 同意后浏览器跳回本机回调地址，终端自动换取并打印：

  ```
  GMAIL_REFRESH_TOKEN=1//0xxxxxx...
  GMAIL_EMAIL=<你的邮箱>
  ```

> refresh token 仅本次显示，立即复制；authorization code 是一次性的，
> 失败重跑整个 `--gmail-auth` 即可。

## ④ CLI 方式启用

仅使用 CLI 备用方式时，将打印的两个值写入 `.env` 并重启服务。设置页授权无需此步骤。验证：

```bash
# 启动日志应出现：邮箱：Gmail（你的邮箱）
grep 邮箱 /tmp/resource-center.log

# API 状态（mock 应为 false）
curl -b cookie.txt http://127.0.0.1:8080/api/v1/mail/gmail/status
```

页面 `#/mailbox` 顶部徽标变为邮箱地址、演示提示条消失，即接入完成。

## 常见问题

| 现象 | 原因与处理 |
|---|---|
| 授权页 `Error 403: access_denied` | OAuth 同意屏的**测试用户**没加你的邮箱，补上重试 |
| 「未验证的应用」警告 | 自用测试应用正常，点「继续」 |
| 换 token 报网络超时 | 出站需代理：`HTTPS_PROXY=... ` 前缀执行 `--gmail-auth` |
| 回调端口被占用 | `GMAIL_AUTH_PORT=其他端口 ./resource-center --gmail-auth`（Google 侧无需登记，loopback 任意端口均可） |
| 页面提示 token 失效 | refresh token 被撤销（改密/回收）或长期未用，重跑 `--gmail-auth` |
| 想断开授权 | Google 账号 → 安全 → 第三方应用权限中撤销；或 Google Cloud 删除 OAuth 客户端 |

## 说明

- **权限最小化**：scope 仅 `gmail.readonly`，不能读写的范围一概未申请
- **邮件不落库**：应用只在内存中转发列表/正文，不持久化邮件内容
- **Outlook**：后期按 `mail.Provider` 接口实现（Microsoft Graph OAuth 同款思路），
  页面已有占位 tab
