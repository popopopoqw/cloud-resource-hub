# 完整配置说明

配置读取顺序为：运行进程的环境变量优先，其次是项目根目录的 `.env`，最后才使用程序默认值。

外部服务也可以在“账号设置”页面配置。保存时服务端会先测试连接，再将敏感值加密写入 SQLite，并立即替换运行时客户端，无需重启。

## 基础配置

| 变量 | 默认值 | 说明 |
|---|---|---|
| `PORT` | `:8080` | HTTP 监听地址 |
| `BASE_URL` | 空 | 公网基础地址，Gmail OAuth 回调等场景使用 |
| `AUTH_USER` | `demo` | 站点登录账号；生产环境必须覆盖 |
| `AUTH_PASS` | `demo123` | 站点登录密码；生产环境必须覆盖 |
| `SQLITE_PATH` | `./resource-center.db` | SQLite 文件；设为 `memory` 时仅使用内存存储 |
| `DATA_DIR` | `./data` | 本地对象、论坛会话等运行数据目录 |

## 123 云盘

| 变量 | 说明 |
|---|---|
| `PAN123_ACCOUNT` / `PAN123_PASSWORD` | 推荐方式；自动登录获取 Token，并在失效时续期 |
| `PAN123_TOKEN` | 直接使用内部 API Token；失效后需手动更新 |
| `PAN123_CLIENT_ID` / `PAN123_CLIENT_SECRET` | 123 云盘开放平台凭据 |
| `PAN123_ACCESS_TOKEN` | 开放平台 Access Token |
| `PAN123_API_BASE` | 开放平台地址，默认 `https://open-api.123pan.com` |

完整接入流程见 [123 云盘配置指南](configure-pan123.md)。

### 影视转存目录

- 电视剧：`影视合集/韩剧`、`影视合集/国产剧`、`影视合集/美剧`
- 未识别地区的电视剧：`影视合集/未分类`
- 电影：`影视合集/电影`
- 综艺：`影视合集/综艺`

## 论坛资源源

| 变量 | 说明 |
|---|---|
| `RESPOOL_ACCOUNT` / `RESPOOL_PASSWORD` | 论坛账号，用于自动登录和更新持久化会话 |
| `RESPOOL_COOKIE` | 可选的手动 Cookie，优先级高于自动登录会话 |
| `RESPOOL_REPLY_TEXT` | 自动回复解锁资源时使用的文案 |
| `RESPOOL_HEADED` | `true` 时显示浏览器窗口，便于人工完成验证 |

论坛登录状态会保存到 `DATA_DIR`，正常情况下无需反复填写 Cookie。

## Gmail

| 变量 | 说明 |
|---|---|
| `GMAIL_CLIENT_ID` / `GMAIL_CLIENT_SECRET` | Google OAuth 客户端 |
| `GMAIL_REFRESH_TOKEN` | Gmail Refresh Token |
| `GMAIL_EMAIL` | Gmail 邮箱地址 |

推荐在设置中心完成 OAuth 授权。详细步骤见 [Gmail 配置指南](configure-gmail.md)。

## Cloudflare 与 R2

| 变量 | 说明 |
|---|---|
| `CF_ACCOUNT_ID` | Cloudflare Account ID，监控和 R2 共用 |
| `CF_API_TOKEN` | Cloudflare 只读 API Token |
| `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` | R2 S3 兼容凭据 |
| `R2_BUCKET` | 默认 Bucket |
| `R2_PUBLIC_BASE` | 可选的公开访问地址 |

未配置 R2 时，书籍文件存入 `DATA_DIR` 下的本地目录。

## Z-Library

| 变量 | 默认值 | 说明 |
|---|---|---|
| `ZLIB_BASE_URL` | `https://z-lib.sk` | 书库入口地址 |
| `ZLIB_MODE` | `jump` | `jump` 为浏览器跳转，`direct` 为服务端直连 |
| `ZLIB_EMAIL` / `ZLIB_PASSWORD` | 空 | 仅 direct 模式使用 |

## 数据持久化与备份

默认 SQLite 持久化以下内容：

- 用户与登录会话
- 转存任务及状态
- 书架和书籍转存记录
- 加密后的外部服务配置
- 功能模块最近一次健康检测结果

建议备份：

```text
resource-center.db
resource-center.db-wal
resource-center.db-shm
data/
```

复制数据库前应停止应用写入，或使用 SQLite 的 `.backup` 命令生成一致性备份。

## 安全建议

- 生产环境必须覆盖默认的 `AUTH_USER` 和 `AUTH_PASS`
- `.env`、SQLite 数据库和 `DATA_DIR` 不应提交到 Git
- Cloudflare 和 Gmail Token 应遵循最小权限原则
- 不要通过日志、截图或 Issue 公开密码、Cookie、Token 和 Secret
