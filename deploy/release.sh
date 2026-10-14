#!/usr/bin/env bash
# ============================================================================
# release.sh — 本机交叉编译 resource-center 并上传到 GCE（Linux amd64）
#
# 用法:
#   ./deploy/release.sh <user@gce-host>                # 常规更新：二进制+配置
#   ./deploy/release.sh <user@gce-host> --with-data    # 首次部署：附带数据库与 data/
#   ./deploy/release.sh --build-only                   # 只交叉编译，不上传
#
# 前提:
#   - 本机在项目根目录可 go build；GCE 目标用户有 sudo（安装服务文件需要）
#   - 隧道凭据（~/.cloudflared/）与 cloudflare/tunnel-config.yml 已由
#     cloudflare-setup.sh 在本机生成过（复用同一隧道）
# 详见 docs/deploy-gce.md
# ============================================================================
set -euo pipefail

APP_DIR="/opt/resource-center"           # GCE 上的部署目录（与 systemd unit 一致）

# ---- 参数解析 ----
TARGET=""
WITH_DATA=false
BUILD_ONLY=false
for a in "$@"; do
  case "$a" in
    --with-data) WITH_DATA=true ;;
    --build-only) BUILD_ONLY=true ;;
    -h|--help) sed -n '2,15p' "$0"; exit 0 ;;
    *) TARGET="$a" ;;
  esac
done
if ! $BUILD_ONLY && [ -z "$TARGET" ]; then
  sed -n '2,15p' "$0"; exit 1
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/resource-center-linux"

# ---- 1. 交叉编译（纯 Go 依赖：modernc SQLite 免 CGO，chromedp/utls 均可交叉）----
echo "==> 交叉编译 linux/amd64 ..."
cd "$ROOT"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$BIN" .
echo "    $(du -h "$BIN" | cut -f1)  $BIN"
$BUILD_ONLY && { echo "==> --build-only，结束"; exit 0; }

# ---- 2. 上传应用 ----
# 首次由 gce-init.sh 建好目录与属主；rsync 用 sudo 保权限（rc 用户拥有部署目录）
RS="sudo rsync"   # 远端经 --rsync-path 以 root 接收
echo "==> 上传应用到 ${TARGET}:${APP_DIR} ..."
# 首次部署时 rc 用户尚不存在（gce-init.sh 也会建，这里保证先于 chown 存在）
ssh "$TARGET" "sudo mkdir -p $APP_DIR/deploy $APP_DIR/cloudflare; id rc >/dev/null 2>&1 || sudo useradd -r -m -s /usr/sbin/nologin rc; sudo chown -R rc:rc $APP_DIR"
rsync -avz --rsync-path="$RS" "$BIN" "$TARGET:$APP_DIR/resource-center"
rsync -avz --rsync-path="$RS" \
  "$ROOT/deploy/resource-center.service" "$ROOT/deploy/gce-init.sh" "$TARGET:$APP_DIR/deploy/"
rsync -avz --rsync-path="$RS" "$ROOT/cloudflare/tunnel-config.yml" "$TARGET:$APP_DIR/cloudflare/"

# .env：有则上传（含全部凭据；GCE 海外直连，确认 HTTPS_PROXY 已注释）
if [ -f "$ROOT/.env" ]; then
  rsync -avz --rsync-path="$RS" "$ROOT/.env" "$TARGET:$APP_DIR/.env"
  ssh "$TARGET" "sudo chmod 600 $APP_DIR/.env && sudo chown rc:rc $APP_DIR/.env"
fi

# ---- 3. 首次部署：数据 + 隧道凭据 ----
if $WITH_DATA; then
  echo "==> 附带数据（SQLite + data/）..."
  for f in resource-center.db resource-center.db-wal resource-center.db-shm; do
    [ -f "$ROOT/$f" ] && rsync -avz --rsync-path="$RS" "$ROOT/$f" "$TARGET:$APP_DIR/"
  done
  # chrome-profile 是论坛 Chrome 自动登录（已废弃死代码）的遗留产物，服务端不用
  rsync -avz --rsync-path="$RS" --exclude=chrome-profile "$ROOT/data/" "$TARGET:$APP_DIR/data/"
  # 注意：带库迁移前先停本机应用，避免 SQLite 写入中拷贝导致库不一致
  pgrep -f './resource-center' >/dev/null && \
    echo "    !! 本机应用仍在运行，建议停止后重跑以保证库文件一致"
fi

# 隧道凭据：复用本机隧道（上传后由 gce-init.sh 安装到 /etc/cloudflared/）
CF_DIR="$HOME/.cloudflared"
if [ -d "$CF_DIR" ]; then
  TUNNEL_ID="$(awk '/^tunnel:/{print $2}' "$ROOT/cloudflare/tunnel-config.yml" 2>/dev/null || true)"
  if [ -n "$TUNNEL_ID" ] && [ -f "$CF_DIR/$TUNNEL_ID.json" ]; then
    echo "==> 上传隧道凭据（${TUNNEL_ID}）..."
    ssh "$TARGET" "sudo mkdir -p /etc/cloudflared && sudo chmod 700 /etc/cloudflared"
    rsync -avz --rsync-path="sudo rsync" \
      "$CF_DIR/$TUNNEL_ID.json" "$CF_DIR/cert.pem" "$TARGET:/etc/cloudflared/" ||
      { echo "    !! 凭据上传失败：可手动 scp 到 ${TARGET}:/etc/cloudflared/"; }
  fi
fi

# 上传统一属主（sudo 接收的文件默认 root 属主；应用以 rc 运行需可写 db/data）
ssh "$TARGET" "sudo chown -R rc:rc $APP_DIR"

echo "==> 上传完成。远端执行初始化/重启："
echo "    首次:  ssh $TARGET 'sudo bash $APP_DIR/deploy/gce-init.sh'"
echo "    更新:  ssh $TARGET 'sudo systemctl restart resource-center'"
