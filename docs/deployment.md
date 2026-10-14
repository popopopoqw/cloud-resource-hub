# 部署总览

## Docker

```bash
docker build -t resource-center .
docker run -d \
  --name resource-center \
  -p 8080:8080 \
  -e AUTH_USER=admin \
  -e AUTH_PASS='请替换为高强度密码' \
  -v resource-center-data:/data \
  resource-center
```

容器内 SQLite、本地对象存储和论坛会话统一写入 `/data`。正式环境必须挂载持久化卷。

发布标签后也可以直接使用 GitHub Container Registry 镜像：

```bash
docker pull ghcr.io/<owner>/<repository>:latest
```

## Cloudflare Tunnel

应用仍需运行在本机、VPS、GCE 或容器环境中，Cloudflare Tunnel 负责公网入口：

```bash
cloudflared tunnel create resource-center
cloudflared tunnel --config cloudflare/tunnel-config.example.yml run resource-center
```

一键配置：

```bash
./deploy/cloudflare-setup.sh <子域名>
```

详细说明见 [Cloudflare 部署指南](deploy-cloudflare.md)。

## Linux / GCE 常驻部署

```bash
./deploy/release.sh <user>@<host>
```

脚本会交叉编译 Linux amd64 二进制并上传，配合 systemd 与 Cloudflare Tunnel 常驻运行。首次部署、数据迁移、备份和回退见 [GCE 部署指南](deploy-gce.md)。

## 健康检查

```bash
curl http://127.0.0.1:8080/api/v1/health
```

Dockerfile 和 systemd 部署均使用该接口判断进程是否正常。
