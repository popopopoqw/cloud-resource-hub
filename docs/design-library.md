# 书库模块（Z-Library 对接）设计方案

> 状态：**已决策，实施中**。2026-09-29 拍板：
> - **D1 对接深度**：L3 变体 —— 搜索 + 下载 + 转存到 **Cloudflare R2**（对象存储），
>   下载的电子书落入 R2，从应用按记录读取
> - **D2 凭据方式**：**账号密码自动登录**（`ZLIB_EMAIL`/`ZLIB_PASSWORD` 先配置化，
>   真实凭据后期提供；提供前走 Mock 演示链路）
> - **D3 书架持久化**：**SQLite**（modernc.org/sqlite，免 CGO），
>   任务/账号/会话等全部数据一并落盘

## 0. 调研结论（设计依据）

| 事实 | 影响 |
|---|---|
| z-lib.sk 是 Z-Library 统一**登录授权页**，未登录无法搜索/浏览任何内容 | 对接必须带凭据（或纯跳转模式） |
| 登录成功后跳转到**个人专属域名**（域名不固定） | 服务端需跟随重定向并记住最终域名 |
| **无公开官方 API**，可用能力是对 web 端点的逆向：搜索 `/s/{query}`、详情 `/book/{id}`、下载 `/dlp/{id}/{hash}` | 实现为 HTML 解析 + 会话保持，接口随时可能变动 |
| Z-Library 域名体系**频繁漂移**（z-lib.sk / z-lib.li / z-lib.ac …）且有发布页 | 基础 URL 必须可配置，内置健康检查 |
| 下载有**免费额度限制**（约 10 本/天，Premium 更多） | 单账号全站共享额度，需额度提示与用量展示 |
| 从当前本机网络直连 z-lib.sk 超时 | 部署服务器需可达（Cloudflare 海外出口没问题） |
| 内容绝大多数为未授权电子书（影子图书馆） | 见 §6 合规边界，自用风险自担 |

## 1. 功能定位

概览页新增「书库」模块卡片，进入书库模块：

```
书库
├── 搜索：书名 / 作者 / ISBN；过滤（语言、格式 epub|pdf|mobi、排序、分页）
├── 结果：封面卡片流（标题、作者、年份、格式、大小、评分）
├── 详情：封面大图 + 出版信息 + 简介 + 动作按钮
├── 书架：收藏夹 + 下载历史（按用户隔离）
└── 动作（视对接深度）：下载到本地 / 转存到网盘 / 收藏
```

## 2. 对接深度（决策点 D1，三档）

| 档位 | 能力 | 需要凭据 | 工作量 | 备注 |
|---|---|---|---|---|
| **L1 站内搜索 + 跳转** | 站内搜索展示结果，点击「下载」新开窗口跳转 Z-Library 原站 | ❌ | 小（1 个解析器） | 搜索也要登录 → 实际仍需服务端凭据，或降级为「跳转原站自行搜索」 |
| **L2 搜索 + 代理下载**（推荐） | L1 + 服务端拿下载直链，流式代理给浏览器下载 | ✅ | 中 | 下载走服务器带宽；文件不落盘（流式转发） |
| **L3 完整闭环** | L2 + 下载后自动上传到用户 123云盘（"找书→进网盘"） | ✅ + 123pan 凭据 | 大 | 依赖 123pan 开放平台**上传**接口（目前转存接口也是 TODO，需一并啃下） |

> 推荐 L2 起步：架构上 L3 只是 L2 之后再接一条上传管道，后续升级不用重做。

## 3. 架构设计（复用现有模式）

### 3.1 对接层（新包 `internal/book/`）

```go
// 与 pan.Provider 平行的抽象
type BookProvider interface {
    Search(ctx, Query) (*SearchResult, error)      // 关键词 + 过滤 + 分页
    Detail(ctx, bookID) (*Book, error)             // 详情 + 下载直链
    DownloadURL(ctx, bookID) (string, error)       // L2+ 使用
}

type ZLib struct {          // internal/book/zlib.go
    baseURL  string         // ZLIB_BASE_URL，可配置应对域名漂移
    client   *http.Client   // 带 cookie jar，保持登录会话
    email, pass string      // 方案 A：账号密码自动登录
    cookieRaw string        // 方案 B：直接注入登录后 cookie（推荐，跳过登录逆向）
}
```

- Mock 实现（`book/mock.go`）与 `PAN_MOCK` 共用开关，演示模式不外联
- 搜索结果服务端缓存（TTL 10 分钟，LRU 上限 500 条）：降低请求频率、应对反爬
- 解析器写成纯函数 + 离线 HTML fixture 单测，站点改版时只改解析器

### 3.2 数据层（store 扩展，内存实现与现有一致）

```go
ShelfItem  {ID, UserID, BookID, Title, Authors, Cover, Format, Size, AddedAt}   // 收藏
BookRecord {ID, UserID, BookID, Title, Format, Size, Status, CreatedAt}         // 下载历史
```

### 3.3 API（挂 `/api/v1`，登录墙内）

```
GET  /library/search?q=&page=&lang=&ext=&sort=
GET  /library/books/{id}
GET  /library/books/{id}/download      # L2：302/流式代理
GET  /library/shelf                    # 书架（收藏）
POST /library/shelf  DELETE /library/shelf/{id}
GET  /library/history  DELETE /library/history/{id}
GET  /library/status                   # 登录态 + 今日剩余额度（解析页面额度展示）
```

### 3.4 前端

- `MODULES` 注册表加「书库」卡片
- 新路由 `#/library`：搜索栏 + 过滤器 + 封面卡片网格 + 详情抽屉 + 书架 tab
- 下载中任务复用现有进度条/轮询交互模式

### 3.5 配置

```
ZLIB_BASE_URL    # 默认 https://z-lib.sk，域名漂移时改这里
ZLIB_EMAIL / ZLIB_PASSWORD   # 凭据方案 A
ZLIB_COOKIE      # 凭据方案 B（推荐）：浏览器登录后复制 rem_id 等_COOKIE，直接注入
ZLIB_CACHE_TTL   # 搜索缓存，默认 10m
```

## 4. 凭据方式（决策点 D2）

| 方式 | 说明 | 优缺点 |
|---|---|---|
| A 账号密码 | 服务端模拟登录表单，自动跟随跳转到个人域名 | 全自动；但登录流程/验证码改版即失效，逆向面大 |
| **B Cookie 注入（推荐）** | 你在浏览器登录一次，把 Cookie 配到环境变量 | 稳、实现小；过期需手动更新（免费号约 30 天） |
| C 无凭据 | 仅支持 L1 的「跳转原站自行搜索」降级模式 | 零维护但体验差 |

## 5. 书架持久化（决策点 D3）

| 方式 | 说明 |
|---|---|
| 内存（推荐先行） | 与任务/账号一致，重启即空；实现零成本 |
| SQLite（modernc，免 CGO） | 收藏不丢；引入第一个第三方依赖 + 单文件卷持久化 |
| Cloudflare D1/KV | 上 CF 时的终态，接口已抽象好，届时一并做 |

## 6. 合规与风险边界（务必知悉）

1. **版权**：Z-Library 内容绝大多数未经授权。本模块定位为**个人自用工具**，沿用「本站不存储、不传播任何文件」声明（书库代理下载为流式转发、不落盘）；若公开部署，由使用者自行承担内容合规责任
2. **站点风险**：域名漂移（→ 可配置 + 健康检查）、反爬升级（→ 解析器隔离、缓存减压）、限流（→ 额度展示 + 失败提示）
3. **账号风险**：频繁机器行为可能导致账号被限；建议搜索缓存 + 合理 QPS（≥1s 间隔）

## 7. 实施拆分（选定方案后）

1. `internal/book/`：接口 + Mock + ZLib 客户端（登录/搜索/详情/下载直链）+ 解析器单测
2. store 扩展：书架/历史（用户隔离）
3. API handlers + 路由
4. 前端：模块卡片 + 书库页（搜索/结果/详情/书架）
5. 配置 + README + 浏览器联调（Mock 先行，真实凭据后补）
