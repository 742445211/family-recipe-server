# AGENTS.md — family-recipe-server

家庭菜谱 & 点菜 API 后端。配对前端仓库：`742445211/family-recipe-miniapp`。

## 技术栈

- Go 1.22+、Gin、GORM、MySQL
- 配置：`config.yaml`（敏感信息勿提交公开仓库，参考 `config.yaml.example`）
- 模块名：`recipe-server`
- 生产 API：`https://www.zzzjc.xin/api`

## 目录结构

```
main.go                        # 入口：配置、迁移、路由、WebSocket、worker
config/
  config.go                    # 配置结构体与默认值
config.yaml                    # 运行时配置（含 notification 块）
migrations/                    # SQL 迁移（001 初始化，002 通知表）
internal/
  handler/                     # Gin HTTP 处理器
  service/                     # 业务逻辑
    notifier/                  # 通知通道：websocket/wechat_subscribe/wecom_workbench/server_chan/bark/ntfy
    wechattoken/               # 微信/企微 access_token 缓存
  model/                       # GORM 实体
  middleware/                  # JWT 鉴权 AuthRequired
pkg/jwt/                       # JWT 签发与解析
```

## 分层约定

- **handler**：参数绑定、HTTP 状态码、`{"code":0,"data":...}` 响应格式
- **service**：业务规则、数据库操作、异步通知
- **model**：表结构 + `TableName()`；软删除用 `gorm.DeletedAt`
- 认证头：`Authorization: Bearer <token>`；context 中取 `user_id`、`family_id`

## 核心业务

- **用户**：微信 `code` 登录，JWT 有效期见 `jwt.expire_hours`
- **家庭**：邀请码加入；`family_members.is_chef` 标识厨师
- **点菜**：`daily_orders` 按 `date` + `meal_type`（breakfast/lunch/dinner）；同餐次同菜不可重复
- **通知**（点菜成功后异步）：
  1. 写入 `notifications`（强一致）
  2. 调度通道写 `notification_deliveries`
  3. WebSocket 在线推送；外部通道离线补充

## 配置分层


| 层级  | 位置                               | 内容                                       |
| --- | -------------------------------- | ---------------------------------------- |
| 平台级 | `config.yaml` → `notification.`* | 开关、企微 corp 凭证、重试策略、API 基址、模板 ID          |
| 用户级 | `notification_channels` 表        | SendKey、Bark key、ntfy topic、wecom userid |
| 运行时 | 内存                               | access_token 缓存                          |


新增通知通道时：在 `config.go` + `config.yaml.example` 预留平台配置，在 `notifier/` 实现 `Notifier` 接口，并扩展 `notification_channels` 的 `channel` 枚举。

## API 响应约定

```json
{ "code": 0, "msg": "ok", "data": {} }
{ "code": 400, "msg": "错误说明" }
{ "code": 401, "msg": "未登录" }
```

业务错误用 400 + `code` 字段；401 由 `middleware.AuthRequired` 返回。

## AI 推荐（结构化 + Redis）

- `POST /api/ai/recommend` → `{ batch_id, items, rate_limit }`；每人 3h 最多 3 次（429）
- `GET /api/ai/items/:item_id` — 从 Redis 读草稿
- `POST /api/ai/items/:item_id/import-recipe` / `add-order` — 从 Redis 入库/点菜
- `GET /api/weather` — Open-Meteo，默认成都，缓存 3h
- 配置：`redis`、`weather`、`ai.rate_limit` 见 `config.yaml.example`

## 测试（严格 TDD，强制）

**所有新功能、行为变更与 Bug 修复必须严格遵循 TDD**：先写（或补）失败测试 → 再写实现 → 再跑通测试。禁止先写实现后补测试应付检查。

### 必须遵守

1. **Red**：新增/修改行为前，先添加能表达预期行为的失败测试（或先跑现有测试确认失败）。
2. **Green**：以最小实现让测试通过。
3. **Refactor**：在测试保护下整理代码，改后必须再次 `go test` 全绿。
4. **覆盖范围**：`service`、`handler`、`middleware`、`config`、`pkg/*`、`notifier`、`wechattoken` 等包中新增/变更的导出逻辑或分支，必须有对应单元测试；纯 HTTP 转发且无分支的 handler 也至少覆盖参数校验与鉴权路径。
5. **测试复用**：内存 DB 与种子数据统一使用 `internal/testutil`（`SetupTestDB`、`SeedUserAndFamily`、`EnsureAppConfig` 等），避免各包重复造轮子。
6. **外部依赖**：微信/AI/天气/OSS 等 HTTP 调用在测试中必须用 `httptest` 或注入 mock client，禁止依赖生产密钥或真实外网（OSS 集成测试无配置时 `t.Skip`）。

### 测试位置

| 包 | 测试文件 |
| --- | --- |
| 业务逻辑 | `internal/service/*_test.go` |
| HTTP 处理器 | `internal/handler/*_test.go` |
| 中间件 | `internal/middleware/*_test.go` |
| 配置 | `config/*_test.go` |
| 工具库 | `pkg/**/*_test.go` |
| 通知通道 | `internal/service/notifier/*_test.go` |
| 微信 token | `internal/service/wechattoken/*_test.go` |
| 测试辅助 | `internal/testutil/`（非 `*_test.go`，仅供测试包 import） |

### 运行命令

```bash
go mod tidy
GOTOOLCHAIN=go1.24.0 CGO_ENABLED=1 go build .
# 快速（无 CGO 依赖的包）：
GOTOOLCHAIN=go1.24.0 go test ./internal/service/notifier/... ./pkg/... ./config/... ./internal/middleware/... -count=1
# Linux 全量（含 SQLite 内存库集成测试，需 CGO）：
GOTOOLCHAIN=go1.24.0 CGO_ENABLED=1 go test ./... -count=1
```

- 测试 DB：`testutil.SetupTestDB()` 使用内存 SQLite（需 CGO）；Windows 无 CGO 时集成测试可能失败，在 Linux CI/服务器执行全量测试。
- 纯 notifier / config / jwt 测试不依赖 DB，应始终可通过。

## 编码规范

- 中文注释说明非显而易见的业务逻辑
- 点菜成功与通知外发解耦：handler 先返回响应，再 goroutine 调 `NotifyOrderCreated`
- 外部通道 secret **不得**在 API 响应或日志中明文输出；返回 `masked_target`
- 改动范围最小化；匹配现有 handler/service 命名风格

## 部署

**强制流程**：本地改代码 → `git commit` + `git push` → 服务器 `git pull` → `go build` → `systemctl restart recipe-server`

- 服务器路径：`/root/projects/recipe-server`（systemd 服务名 `recipe-server`）
- 禁止 SFTP 直传文件代替 git 同步（紧急热修后须补提交）
- Ubuntu 24.04 步骤见前端仓库 `docs/deploy-ubuntu-24.04.md`
- 迁移：`migrations/002_notifications.sql` + GORM `AutoMigrate`
- Nginx 必须为 `/api/ws` 配置 WebSocket `Upgrade` 头
- 企微：服务器 IP 加入企业可信 IP；厨师关注微信插件

## 相关文档

- 通知方案（前后端共同依据）：`family-recipe-miniapp/docs/chef-notification-plan.md`
- 部署命令：`family-recipe-miniapp/docs/deploy-ubuntu-24.04.md`

## 常见改动入口


| 任务     | 主要文件                                                    |
| ------ | ------------------------------------------------------- |
| 新增 API | `internal/handler/*.go` + `main.go` 路由                  |
| 点菜逻辑   | `internal/service/order.go`、`internal/handler/order.go` |
| 通知调度   | `internal/service/notification.go`                      |
| 新通知通道  | `internal/service/notifier/<channel>.go`                |
| 配置项    | `config/config.go`、`config.yaml.example`                |


