# GCE + Cloudflare Tunnel 部署指南

> 目标：把 resource-center 从本机迁到 GCE 云实例（2C1G）常驻运行，
> 继续经 Cloudflare Tunnel 对外服务——无需公网入站端口，域名/DNS/隧道全部复用。

## 为什么是这个形态

- **代码零改动**：Go 交叉编译一次，`.env` 原样搬，SQLite 数据文件直接复制。
- **GCE 海外区域出站直连**：pan1.me / 123pan / api.cloudflare.com / z-lib 全部无墙，
  `.env` 里本地代理（`HTTPS_PROXY`）注释掉即可。
- **入口仍在 Cloudflare**：Google IP 国内不可直连，Tunnel 反向出站连接正好绕开，
  手机等无代理设备照常访问 `https://<域名>`。
- **常驻**：systemd 管理应用与 cloudflared，开机自启、崩溃自动拉起。

## 资源评估（2C1G）

常态内存 300–500MB（系统 150–250 + 应用 80–300 + cloudflared 30–50），
够用但不宽裕，两条保险：

- `gce-init.sh` 自动创建 **2G swap**（vm.swappiness=10，只做兜底）；
- systemd 对应用设了 `MemoryMax=512M`，抓取高峰内存失控时只重启应用，
  不把 cloudflared 一起 OOM。

> 注意：论坛 Chrome 自动登录代码（`internal/respool/session.go`）是死代码，
> 运行时不会启动 Chrome。不要在 1G 内存机器上恢复该路径。

## 前提

1. **GCE 实例**：Debian 12 / Ubuntu 22.04+，2C1G（e2-custom 或近似），10G 磁盘起步。
   - 可选无外网 IP + Cloud NAT（仅出站）——cloudflared 只需出站，更省钱更安全；
   - 防火墙**不需要任何入站规则**。
2. **本机已跑过** `deploy/cloudflare-setup.sh`：隧道已创建、`cloudflare/tunnel-config.yml`
   与 `~/.cloudflared/<tunnel-id>.json` 已存在（迁移即复用这条隧道）。
3. GCE 上目标用户有 sudo（装服务/写 /etc/cloudflared 需要）。

## 部署三步

### 1. 本机：编译 + 上传

```bash
# 常规更新（二进制 + 配置 + .env）
./deploy/release.sh <user>@<gce-ip>

# 首次部署：附带数据库与 data/（论坛会话、对象兜底）
# 先停本机应用，避免拷贝写入中的 SQLite
./deploy/release.sh <user>@<gce-ip> --with-data

# 只想验证交叉编译：
./deploy/release.sh --build-only
```

release.sh 做了什么：交叉编译 `linux/amd64`（纯 Go 依赖，免 CGO）→
rsync 二进制、systemd unit、gce-init.sh、tunnel-config.yml、`.env` 到
`/opt/resource-center/` → `--with-data` 时附带 `resource-center.db*` 与 `data/` →
上传隧道凭据到 `/etc/cloudflared/`。

### 2. GCE：初始化（一次性）

```bash
ssh <user>@<gce-ip>
sudo bash /opt/resource-center/deploy/gce-init.sh
```

幂等地完成：rc 用户与目录属主 → 2G swap → cloudflared（官方 apt 源）→
resource-center systemd 服务（健康检查通过才算成功）→ cloudflared systemd 服务
（凭据路径重写 + `cloudflared service install`）。

### 3. 验证与切换

```bash
# GCE 上本地验证
curl http://127.0.0.1:8080/api/v1/health

# 公网验证（域名/DNS 未变，隧道换了个源站而已）
curl https://<你的域名>/api/v1/health
```

通过后**停掉本机的应用与 cloudflared**——同一隧道 ID 两端同时跑会互相踢连接：

```bash
pkill -f './resource-center'; pkill -f 'cloudflared tunnel'
```

## 日常运维

| 操作 | 命令 |
|---|---|
| 发新版 | 本机 `./deploy/release.sh <user>@<gce-ip>` → `ssh <gce> 'sudo systemctl restart resource-center'` |
| 看应用日志 | `journalctl -u resource-center -f` |
| 看隧道日志 | `journalctl -u cloudflared -f` |
| 重启应用/隧道 | `sudo systemctl restart resource-center / cloudflared` |
| 备份数据 | `resource-center.db` + `data/`（停服后 rsync，或用 `sqlite3 .backup`） |

## 回退

GCE 出问题时，本机拉起应用与隧道即回原状（数据以最新的一端为准，必要时反向 rsync）：

```bash
./resource-center &
cloudflared tunnel --config cloudflare/tunnel-config.yml run resource-center &
```

## 故障排查

| 现象 | 处理 |
|---|---|
| gce-init 健康检查失败 | `journalctl -u resource-center -n 50`；常见：`.env` 没传、8080 被占 |
| 应用正常但公网 502 | `journalctl -u cloudflared -n 50`；确认本机 cloudflared 已停（同隧道互踢） |
| 凭据上传失败 | 手动 `scp ~/.cloudflared/<tunnel-id>.json ~/.cloudflared/cert.pem <gce>:/etc/cloudflared/` |
| 内存吃紧 | `free -h` 看 swap 使用；确认没走到 Chrome 自动登录路径 |
| cloudflared 安装慢/失败 | 检查 apt 源；或从 GitHub releases 下 .deb 手动装 |

## 与其他形态的关系

- **本机 + Tunnel**（原形态）：零成本但依赖本机常开，本方案是其「换台机器」版。
- **Docker**：GCE 上也可用 Dockerfile 跑（`/data` 挂卷），再装 host 级 cloudflared；
  二进制 + systemd 更轻，1G 内存下推荐后者。
- **Cloudflare Containers**：免 GCE 但需 Workers Paid（$5/月起）+ 存储改造，见 README 讨论。
