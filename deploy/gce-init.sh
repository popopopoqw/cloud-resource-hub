#!/usr/bin/env bash
# ============================================================================
# gce-init.sh — GCE 实例一次性初始化（Debian/Ubuntu，root 执行）
#
# 用法（release.sh 上传后，在 GCE 上）:
#   sudo bash /opt/resource-center/deploy/gce-init.sh
#
# 做什么（幂等，可重复执行）:
#   1. 应用用户 rc 与部署目录属主
#   2. 2G swap（1G 内存机器的 OOM 保险）
#   3. cloudflared（Cloudflare 官方 apt 源）
#   4. resource-center systemd 服务（deploy/resource-center.service）
#   5. cloudflared systemd 服务（凭据已在 /etc/cloudflared/，重写配置路径后安装）
# 详见 docs/deploy-gce.md
# ============================================================================
set -euo pipefail

APP_DIR="${APP_DIR:-/opt/resource-center}"
CF_ETC="/etc/cloudflared"

step() { printf '\n\033[1;36m==> %s\033[0m\n' "$1"; }
ok()   { printf '\033[32m%s\033[0m\n' "$1"; }
err()  { printf '\033[31m%s\033[0m\n' "$1"; }

[ "$(id -u)" = 0 ] || { err "请以 root 执行: sudo bash $0"; exit 1; }

# ---- 1. 应用用户与目录 ----
step "[1/5] 应用用户 rc"
id rc >/dev/null 2>&1 || useradd -r -m -s /usr/sbin/nologin rc
mkdir -p "$APP_DIR" && chown -R rc:rc "$APP_DIR"
ok "rc 用户与 $APP_DIR 就绪"

# ---- 2. swap（2C1G 内存保险）----
step "[2/5] swap"
if swapon --show 2>/dev/null | grep -q .; then
  ok "已有 swap: $(swapon --show --noheadings | awk '{print $1, $3}' | tr '\n' ' ')"
else
  fallocate -l 2G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile >/dev/null
  swapon /swapfile
  grep -q '^/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab
  # 低 swappiness：尽量用内存，swap 只做兜底
  sysctl -w vm.swappiness=10 >/dev/null
  grep -q 'vm.swappiness' /etc/sysctl.conf || echo 'vm.swappiness=10' >> /etc/sysctl.conf
  ok "已启用 2G swap（/swapfile，vm.swappiness=10）"
fi

# ---- 3. cloudflared ----
step "[3/5] cloudflared"
if command -v cloudflared >/dev/null; then
  ok "已安装: $(cloudflared --version)"
else
  command -v curl >/dev/null || { apt-get update && apt-get install -y curl; }
  curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg -o /usr/share/keyrings/cloudflare-main.gpg
  echo 'deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared any main' \
    > /etc/apt/sources.list.d/cloudflared.list
  apt-get update && apt-get install -y cloudflared
  ok "已安装: $(cloudflared --version)"
fi

# ---- 4. 应用服务 ----
step "[4/5] resource-center systemd 服务"
[ -x "$APP_DIR/resource-center" ] || { err "缺少 $APP_DIR/resource-center（先跑 release.sh）"; exit 1; }
install -m 644 "$APP_DIR/deploy/resource-center.service" /etc/systemd/system/resource-center.service
systemctl daemon-reload
systemctl enable resource-center >/dev/null
systemctl restart resource-center
sleep 2
if curl -sf http://127.0.0.1:8080/api/v1/health >/dev/null; then
  ok "resource-center 已启动并通过健康检查"
else
  err "健康检查未过，看日志: journalctl -u resource-center -n 50"
  exit 1
fi

# ---- 5. cloudflared 服务 ----
step "[5/5] cloudflared systemd 服务"
CFG_SRC="$APP_DIR/cloudflare/tunnel-config.yml"
[ -f "$CFG_SRC" ] || { err "缺少 ${CFG_SRC}（先跑 release.sh）"; exit 1; }
TUNNEL_ID="$(awk '/^tunnel:/{print $2}' "$CFG_SRC")"
CRED="$CF_ETC/$TUNNEL_ID.json"
if [ ! -f "$CRED" ]; then
  err "缺少隧道凭据 $CRED"
  err "本机执行: rsync -avz ~/.cloudflared/$TUNNEL_ID.json ~/.cloudflared/cert.pem $USER@本机:/tmp/"
  err "再上传:    scp /tmp/$TUNNEL_ID.json /tmp/cert.pem <GCE>:$CF_ETC/"
  exit 1
fi
mkdir -p "$CF_ETC" && chmod 700 "$CF_ETC"
# 配置里 credentials-file 是本机路径 → 重写为 GCE 实际路径
sed -i "s|^credentials-file: .*|credentials-file: $CRED|" "$CFG_SRC"
chown rc:rc "$CFG_SRC"

# 官方安装方式：--config 全局标志传配置路径（直接传路径会被当作 token 解析）
systemctl stop cloudflared 2>/dev/null || true
cloudflared --no-autoupdate --config "$CFG_SRC" service install
systemctl enable cloudflared >/dev/null
systemctl restart cloudflared
sleep 4
if systemctl is-active --quiet cloudflared; then
  ok "cloudflared 已运行（隧道 ${TUNNEL_ID}）"
else
  err "cloudflared 异常: journalctl -u cloudflared -n 50"
  exit 1
fi

echo
ok "初始化完成。验证公网入口:"
awk '/^BASE_URL=/{print "    " $0}' "$APP_DIR/.env" 2>/dev/null || true
echo "    curl https://<你的域名>/api/v1/health"
