# API 文档 — family-recipe-server

> 生产基址：`https://www.zzzjc.xin/api`  
> 路由注册见 [`main.go`](../main.go)；Handler 注释见 `internal/handler/*.go`。

## 文档索引

| 文件 | 内容 |
|------|------|
| [overview.md](./overview.md) | 通用约定、鉴权、错误码、功能开关 |
| [auth-user.md](./auth-user.md) | 登录、用户信息 |
| [family.md](./family.md) | 家庭与成员 |
| [recipe-category.md](./recipe-category.md) | 家庭菜谱、分类 |
| [catalog.md](./catalog.md) | 全局菜谱库（搜索/生成） |
| [order-favorite.md](./order-favorite.md) | 点菜、收藏、分享 |
| [ai-weather.md](./ai-weather.md) | AI 推荐、天气 |
| [notification-upload-ws.md](./notification-upload-ws.md) | 通知、通知通道、上传、WebSocket |
| [fridge.md](./fridge.md) | 冰箱食材、拍照识别 |

## 接口总览

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/auth/login` | 公开 | 微信登录 |
| GET | `/app/features` | 公开 | 功能开关 |
| GET | `/weather` | 公开 | 天气 |
| GET | `/categories/public` | 公开 | 公开菜谱分类名 |
| GET | `/recipes` | 可选 | 菜谱列表 |
| GET | `/recipes/:id` | 可选 | 菜谱详情 |
| GET | `/users/me` | 必填 | 当前用户 |
| PUT | `/users/me` | 必填 | 更新用户 |
| POST | `/families` | 必填 | 创建家庭 |
| POST | `/families/join` | 必填 | 加入家庭 |
| GET | `/families` | 必填 | 家庭列表 |
| GET | `/families/:id/members` | 必填 | 成员列表 |
| POST | `/families/chef` | 必填 | 切换厨师 |
| GET | `/categories` | 必填 | 家庭分类 |
| POST | `/recipes` | 必填 | 创建菜谱 |
| PUT | `/recipes/:id` | 必填 | 更新菜谱 |
| DELETE | `/recipes/:id` | 必填 | 删除菜谱 |
| POST | `/recipes/:id/cooked` | 必填 | 烹饪次数 +1 |
| GET | `/orders` | 必填 | 点菜列表 |
| POST | `/orders` | 必填 | 点菜 |
| DELETE | `/orders/:id` | 必填 | 取消点菜 |
| POST | `/orders/share` | 必填 | 分享 activity_id |
| GET | `/favorites` | 必填 | 收藏列表 |
| POST | `/favorites/:id` | 必填 | 收藏 |
| DELETE | `/favorites/:id` | 必填 | 取消收藏 |
| POST | `/upload` | 必填 | 上传图片 |
| GET | `/notifications/unread` | 必填 | 未读通知 |
| POST | `/notifications/:id/read` | 必填 | 标记已读 |
| GET | `/notification-channels` | 必填 | 通知通道列表 |
| POST | `/notification-channels` | 必填 | 创建通道 |
| PUT | `/notification-channels/:id` | 必填 | 更新通道 |
| DELETE | `/notification-channels/:id` | 必填 | 删除通道 |
| POST | `/ai/recommend` | 必填 | AI 推荐 |
| GET | `/ai/items/:item_id` | 必填 | AI 草稿详情 |
| POST | `/ai/items/:item_id/import-recipe` | 必填 | AI 草稿入库 |
| POST | `/ai/items/:item_id/add-order` | 必填 | AI 草稿点菜 |
| POST | `/catalog-recipes/lookup` | 必填 | 全局库查/生成 |
| GET | `/catalog-recipes/:id` | 必填 | 全局库详情 |
| POST | `/catalog-recipes/:id/use` | 必填 | 全局库使用计数 |
| GET | `/fridge/items` | 必填 | 冰箱库存列表 |
| POST | `/fridge/items` | 必填 | 手动新增食材 |
| PUT | `/fridge/items/:id` | 必填 | 更新食材 |
| DELETE | `/fridge/items/:id` | 必填 | 删除食材 |
| POST | `/fridge/scans` | 必填 | 拍照识别 |
| GET | `/fridge/scans/:id` | 必填 | 轮询识别结果 |
| POST | `/fridge/scans/:id/confirm` | 必填 | 确认入库 |
| GET | `/api/ws` | Token(query) | WebSocket 通知 |

## 维护说明（Agent / 开发者必读）

**凡新增、修改、删除 HTTP 路由或请求/响应字段，必须在同一 PR/提交中同步更新 `api-doc/`。**

1. 改 `internal/handler/*.go` 与 `main.go` 路由。
2. 更新对应模块 markdown（或 `README.md` 总览表）。
3. 在模块文档末尾更新「变更记录」一行（日期 + 摘要）。
4. 前端联调以 `api-doc` 为准；勿仅改代码不更新文档。

权威来源优先级：`main.go` 路由 > handler 实现 > 本文档。文档与代码冲突时以代码为准并立即修文档。
