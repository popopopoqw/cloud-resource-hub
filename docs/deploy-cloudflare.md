# Cloudflare 部署指南（resource-center）

> 目标：把本机运行的资源中心暴露为 `https://<子域名>` 公网地址——无需公网 IP、
> 无需开放端口，自动获得 HTTPS 与 Cloudflare 边缘防护。

## 方式一：一键脚本（推荐）

```bash
cd /opt/WorkSpace/todo/resource-center

# 首次使用前完成两件事：
brew install cloudflared          # 1. 安装（只需一次）
cloudflared tunnel login          # 2. 账号授权：浏览器选中域名（如 ccwu.cc）→ Authorize

# 一键部署（子域名作为参数）
./deploy/cloudflare-setup.sh smvf.ccwu.cc

# 应用监听非 8080 端口时：
APP_PORT=9090 ./deploy/cloudflare-setup.sh smvf.ccwu.cc
```

脚本做了什么（幂等，可重复运行）：

| 步骤 | 内容 |
|---|---|
| 1 | 前置检查：cloudflared / 授权证书 / 应用二进制 |
| 2 | 创建隧道（已存在同名隧道则复用其 ID 与凭据） |
| 3 | 绑定子域 DNS（`CNAME <tunnel-id>.cfargotunnel.com`） |
| 4 | 生成 `cloudflare/tunnel-config.yml`（ingress → localhost:8080） |
| 5 | 更新 `.env`：`BASE_URL=https://<子域>`；注释 `HTTPS_PROXY`（公网部署无需本地代理） |
| 6 | 重启应用与隧道（nohup 后台），curl 外网健康检查验证 |

成功标志：终端输出 `部署成功：https://<子域名>`。

## 方式二：手动分步（理解每一步时用）

```bash
# 1. 授权（写入 ~/.cloudflared/cert.pem）
cloudflared tunnel login

# 2. 建隧道（凭据写入 ~/.cloudflared/<tunnel-id>.json）
cloudflared tunnel create resource-center

# 3. 绑域名（要求该子域当前无 A/AAAA/CNAME 记录）
cloudflared tunnel route dns resource-center smvf.ccwu.cc

# 4. 配置文件：参照 cloudflare/tunnel-config.example.yml
#    填 tunnel ID、credentials-file、hostname；service 指向 http://localhost:8080

# 5. 运行
cloudflared tunnel --config cloudflare/tunnel-config.yml run resource-center
```

## 常驻（开机自启）

```bash
# 应用（launchd，macOS）或 nohup / systemd（Linux）
# 隧道：安装为系统服务
sudo cloudflared service install <tunnel-config.yml 的绝对路径>
```

Docker 部署时把 `cloudflared` 与应用放进同一 compose（隧道容器访问应用容器名）。

## 故障排查

| 现象 | 原因与处理 |
|---|---|
| `An A, AAAA, or CNAME record with that host already exists` | 子域已有记录：无服务在用 → 控制台删记录后重跑；有服务 → 换子域 |
| 脚本报「未完成账号授权」 | 执行 `cloudflared tunnel login`，浏览器选中域名授权 |
| 外网验证失败但应用正常 | 看隧道日志 `/tmp/cloudflared.log`；刚启动有数秒延迟，稍后重试 |
| 登录后会被踢回登录页 | `.env` 的 `BASE_URL` 未生效（脚本第 5 步）；确认后重启应用 |
| 书库跳转搜索 404/被拦 | 浏览器先登录 Z-Library（jump 模式依赖浏览器侧登录态） |

## 接入真实 Cloudflare 监控数据（可选）

部署后监控页默认显示**演示数据**（页面有黄色提示条、账户显示"演示账户"）。
按以下步骤切换为真实数据：

### 1. 创建只读 API Token

1. 打开 https://dash.cloudflare.com/profile/api-tokens → **Create Token**
2. 使用模板 **View Cloudflare Analytics**；或自定义以下权限（最小集）：

   | 权限 | 级别 |
   |---|---|
   | Account Analytics | Read |
   | Workers Scripts | Read |
   | Zone / Zone Analytics | Read |
   | R2 (可选) | Read |

   Zone Resources 选择 **All zones**（或仅 `smvf.ccwu.cc`）
3. 创建后**立即复制 Token**（只显示一次）

### 2. 获取 Account ID

控制台任意页面右侧栏，或 URL 中：`dash.cloudflare.com/<这一段就是AccountID>/...`

### 3. 写入 .env 并重启

```ini
CF_API_TOKEN=<粘贴token>
CF_ACCOUNT_ID=<账户ID>
```

重启服务后启动日志变为 `Cloudflare 监控：真实账户（...）`；页面提示条消失，
数据变为真实域名（如 smvf.ccwu.cc）的真实访问统计。

### 常见问题

- **部分区块为空**：Token 权限缺项（如域名列表为空 → 缺 Zone:Read）
- **拉取失败**：本机开发需在 `.env` 配 `HTTPS_PROXY`（api.cloudflare.com 国内不可直连）；
  隧道部署在 Cloudflare 边缘出口则无需代理
- **数据保留期**：免费账户日粒度约 30 天

## 接入 R2 对象存储（可选）

书库转存的文件默认落本地 `data/`；配置 R2 后自动切到桶（业务代码零改动）：

1. 控制台 → R2 → **Create bucket**（如 `resource-center`）
2. R2 概览页 → **Manage R2 API Tokens** → 创建（Object Read & Write，限定该桶）
3. `.env` 填四项并重启（启动日志显示 `对象存储：Cloudflare R2`）：

```ini
# R2_ACCOUNT_ID 可省略（未配置时自动使用 CF_ACCOUNT_ID，同一账号）
R2_ACCESS_KEY_ID=<token的Access Key ID>
R2_SECRET_ACCESS_KEY=<token的Secret>
R2_BUCKET=resource-center
```

> 配置 `R2_PUBLIC_BASE`（桶的公开自定义域）后，已转存书籍的下载走直链，
> 不再经应用转发。

## 其他增强

- **边缘缓存**（静态资源）：见 `cloudflare/worker.js` + `wrangler.example.toml`

## 快速验证（无域名临时地址）

```bash
cloudflared tunnel --url http://127.0.0.1:8080
# 输出 https://xxx.trycloudflare.com 临时地址（重启失效，仅测试用）
```
