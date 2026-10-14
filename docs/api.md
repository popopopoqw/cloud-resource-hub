# REST API

API 前缀为 `/api/v1`。除健康检查、登录页和登录接口外，所有页面与 API 都需要有效的 `pt_session` Cookie。

## 认证与概览

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/health` | 进程健康检查，无需登录 |
| `POST` | `/auth/login` | 登录，提交 `username`、`password` |
| `POST` | `/auth/logout` | 退出登录 |
| `GET` | `/auth/me` | 当前用户 |
| `GET` | `/module-health` | 读取数据库中的模块健康状态缓存 |
| `GET` | `/stats` | 当前用户转存统计 |

## 资源发现与转存

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/respool/sources` | 论坛资源站点列表 |
| `GET` | `/respool/list` | 资源广场列表 |
| `GET` | `/respool/cover` | 帖子封面 |
| `GET` | `/respool/thread` | 获取帖子资源信息 |
| `POST` | `/respool/login` | 查询或启动论坛登录流程 |
| `POST` | `/media/transfer` | 论坛资源一键转存 |
| `GET` | `/tasks` | 转存记录 |
| `GET` | `/tasks/{id}` | 转存任务详情 |
| `DELETE` | `/tasks/{id}` | 删除转存记录 |

## 我的网盘

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/drive/list` | 分页获取目录和文件 |
| `GET` | `/drive/tree` | 获取目录树 |
| `GET` | `/drive/capacity` | 获取容量信息 |
| `GET` | `/drive/dirsize/{file_id}` | 获取目录大小 |
| `GET` | `/drive/download/{file_id}` | 获取文件下载地址 |
| `POST` | `/drive/mkdir` | 新建目录 |
| `POST` | `/drive/rename` | 重命名 |
| `POST` | `/drive/move` | 移动文件或目录 |
| `POST` | `/drive/trash` | 移入回收站 |

## 邮箱、书库与 Cloudflare

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/mail/providers` | 邮箱供应商列表 |
| `GET` | `/mail/{provider}/messages` | 邮件列表与搜索 |
| `GET` | `/mail/{provider}/messages/{id}` | 邮件详情 |
| `GET` | `/library/search` | 书籍搜索 |
| `GET` | `/library/shelf` | 书架列表 |
| `GET` | `/library/history` | 书籍转存记录 |
| `GET` | `/cfmon/status` | Cloudflare 配置状态 |
| `GET` | `/cfmon/overview` | Cloudflare 监控概览 |

## 设置中心

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/settings/integrations` | 获取已配置服务及脱敏字段 |
| `POST` | `/settings/integrations/test` | 测试候选配置 |
| `POST` | `/settings/integrations` | 测试通过后保存并热生效 |
| `POST` | `/settings/r2/buckets` | 获取可用 R2 Bucket |
| `POST` | `/settings/gmail/oauth/start` | 启动 Gmail OAuth |
