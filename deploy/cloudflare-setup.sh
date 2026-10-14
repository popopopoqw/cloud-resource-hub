#!/usr/bin/env bash
# ============================================================================
# resource-center · Cloudflare Tunnel 一键部署脚本
#
# 用法:
#   ./deploy/cloudflare-setup.sh <子域名>            # 例: ./deploy/cloudflare-setup.sh smvf.ccwu.cc
#   APP_PORT=9090 ./deploy/cloudflare-setup.sh xxx   # 应用监听非 8080 时
#
# 前置条件:
#   1. 域名已托管在 Cloudflare（如 ccwu.cc）
#   2. 已安装 cloudflared:  brew install cloudflared
#   3. 已完成账号授权:      cloudflared tunnel login
#      （首次运行本脚本会检测并提示）
#
# 脚本幂等：隧道已存在则复用、DNS 已绑定则跳过、可重复运行。
# 详见 docs/deploy-cloudflare.md
# ============================================================================
set -euo pipefail

# BSD(macOS) 与 GNU(Linux) 的 sed -i 语法差异：macOS 需显式空备份后缀
if [ "$(uname)" = "Darwin" ]; then SED_I=(sed -i ''); else SED_I=(sed -i); fi

SUBDOMAIN="${1:?用法: $0 <子域名>  例: $0 smvf.ccwu.cc}"
APP_PORT="${APP_PORT:-8080}"
TUNNEL_NAME="${TUNNEL_NAME:-resource-center}"
CF_ZONE="${CF_ZONE:-smvf.ccwu.cc}"  # 授权的 Cloudflare zone（login 时在浏览器选择的域名；不符时用 CF_ZONE=<zone> 覆盖）
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CF_DIR="$HOME/.cloudflared"
CONFIG="$ROOT/cloudflare/tunnel-config.yml"

c_green() { printf '\033[32m%s\033[0m\n' "$1"; }
c_red()   { printf '\033[31m%s\033[0m\n' "$1"; }
c_step()  { printf '\n\033[1;36m==> %s\033[0m\n' "$1"; }

# ---- 1. 前置检查 ----
c_step "[1/6] 前置检查"
command -v cloudflared >/dev/null || {
  hint="sudo apt-get install -y cloudflared  # 官方源安装见 https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/"
  [ "$(uname)" = "Darwin" ] && hint="brew install cloudflared"
  c_red "未安装 cloudflared，请先: $hint"; exit 1;
}
[ -f "$CF_DIR/cert.pem" ] || { c_red "未完成账号授权，请先执行: cloudflared tunnel login"; exit 1; }
[ -x "$ROOT/resource-center" ] || { c_red "未找到应用二进制（$ROOT/resource-center），请先 go build"; exit 1; }
c_green "cloudflared / 授权证书 / 应用二进制 就绪"

# ---- 1.5 域名归属校验 ----
# cloudflared 对不属于授权 zone 的域名不报错，而是把它整个当作记录名
# 绑到授权 zone 下（产生 xxx.other.com.myzone.com 垃圾记录），必须前置拦截。
case "$SUBDOMAIN" in
  "$CF_ZONE"|*."$CF_ZONE") : ;;  # 主域或其子域，合法
  *) c_red "!! $SUBDOMAIN 不在授权 zone（${CF_ZONE}）内"
     c_red "   Cloudflare Tunnel 只能绑定托管在你 CF 账号里的域名。"
     c_red "   若域名确实已加入 CF，用 CF_ZONE=<你的域> 重跑；否则换 ${CF_ZONE} 下的子域。"
     exit 2 ;;
esac

# ---- 2. 隧道（已存在则复用） ----
c_step "[2/6] 创建/复用隧道 $TUNNEL_NAME"
TUNNEL_ID="$(cloudflared tunnel list 2>/dev/null | awk -v n="$TUNNEL_NAME" '$2==n {print $1; exit}')"
if [ -n "$TUNNEL_ID" ]; then
  c_green "复用已有隧道: $TUNNEL_NAME ($TUNNEL_ID)"
else
  TUNNEL_ID="$(cloudflared tunnel create "$TUNNEL_NAME" 2>&1 | grep -oE '[0-9a-f-]{36}' | head -1)"
  [ -n "$TUNNEL_ID" ] || { c_red "隧道创建失败"; exit 1; }
  c_green "已创建隧道: $TUNNEL_NAME ($TUNNEL_ID)"
fi
CRED="$CF_DIR/$TUNNEL_ID.json"
[ -f "$CRED" ] || { c_red "隧道凭据文件缺失: $CRED"; exit 1; }

# ---- 3. DNS 绑定 ----
c_step "[3/6] 绑定域名 $SUBDOMAIN → 隧道"
if cloudflared tunnel route dns "$TUNNEL_NAME" "$SUBDOMAIN" 2>&1 | tee /tmp/route_dns.out; then
  c_green "DNS 绑定成功"
else
  if grep -q "already exists" /tmp/route_dns.out; then
    c_red "!! $SUBDOMAIN 已有 A/AAAA/CNAME 记录，无法自动绑定。"
    c_red "   处理方式二选一："
    c_red "   A. 该子域无服务在用 → Cloudflare 控制台删除旧记录后重跑本脚本"
    c_red "   B. 有服务在用 → 换个子域: $0 <新子域名>"
    exit 2
  elif grep -qi "already routed\|success" /tmp/route_dns.out; then
    c_green "DNS 已绑定（跳过）"
  else
    c_red "DNS 绑定失败，见上方输出"; exit 2
  fi
fi

# ---- 4. 生成隧道配置 ----
c_step "[4/6] 写入隧道配置 $CONFIG"
cat > "$CONFIG" <<EOF
# 由 deploy/cloudflare-setup.sh 生成；手动调整后可直接 cloudflared tunnel --config 本文件 run
tunnel: $TUNNEL_ID
credentials-file: $CRED
protocol: http2

ingress:
  - hostname: $SUBDOMAIN
    service: http://localhost:$APP_PORT
    originRequest:
      connectTimeout: 30s
  - service: http_status:404
EOF
c_green "隧道配置已写入"

# ---- 5. 应用 .env（BASE_URL + 代理处理） ----
c_step "[5/6] 更新 .env（BASE_URL=https://${SUBDOMAIN}）"
ENV_FILE="$ROOT/.env"
if [ -f "$ENV_FILE" ]; then
  # BASE_URL：有则替换、无则追加
  if grep -q '^BASE_URL=' "$ENV_FILE"; then
    "${SED_I[@]}" "s|^BASE_URL=.*|BASE_URL=https://$SUBDOMAIN|" "$ENV_FILE"
  else
    printf '\nBASE_URL=https://%s\n' "$SUBDOMAIN" >> "$ENV_FILE"
  fi
  # 部署到公网后本地代理通常不可达；jump 模式服务端无外联需求 → 注释掉
  "${SED_I[@]}" 's|^HTTPS_PROXY=|# HTTPS_PROXY=（公网部署无需本地代理，jump 模式服务端不外联）|' "$ENV_FILE"
  c_green ".env 已更新"
else
  c_red "未找到 .env（${ENV_FILE}），请先创建"; exit 1
fi

# ---- 6. 启动：应用 + 隧道 ----
c_step "[6/6] 重启应用与隧道"
pkill -f './resource-center' 2>/dev/null || true
sleep 0.5
cd "$ROOT"
nohup ./resource-center > /tmp/resource-center.log 2>&1 &
sleep 1.5
curl -sf "http://127.0.0.1:$APP_PORT/api/v1/health" >/dev/null && c_green "应用已启动（:${APP_PORT}）" || { c_red "应用启动失败，看 /tmp/resource-center.log"; exit 3; }

pkill -f "cloudflared tunnel --config $CONFIG" 2>/dev/null || true
sleep 0.5
nohup cloudflared tunnel --config "$CONFIG" run "$TUNNEL_NAME" > /tmp/cloudflared.log 2>&1 &
sleep 4

echo
if curl -sf "https://$SUBDOMAIN/api/v1/health" >/dev/null 2>&1; then
  c_green "============================================"
  c_green " 部署成功：https://$SUBDOMAIN"
  c_green "============================================"
else
  c_red "外网验证未通过（隧道刚启动可能有几秒延迟）"
  c_red "稍后手动验证: curl https://$SUBDOMAIN/api/v1/health"
  c_red "隧道日志: /tmp/cloudflared.log"
fi
