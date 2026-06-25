# 通用约定

## 基址与协议

- 生产：`https://www.zzzjc.xin/api`
- 本地默认：`http://localhost:8080/api`
- 内容类型：`application/json`（上传接口除外）

## 鉴权

| 类型 | 说明 |
|------|------|
| 公开 | 无需 Header |
| 可选 | `Authorization: Bearer <token>` 有则解析用户/家庭，无则继续 |
| 必填 | 无 token 或无效 → HTTP 401，`{"code":401,"msg":"未登录"}` |

JWT 载荷含 `user_id`、`family_id`（当前家庭）。登录后请求需带：

```http
Authorization: Bearer <token>
```

## 统一响应

成功：

```json
{ "code": 0, "msg": "ok", "data": {} }
```

业务错误（HTTP 400）：

```json
{ "code": 400, "msg": "错误说明" }
```

未登录（HTTP 401）：

```json
{ "code": 401, "msg": "未登录" }
```

功能关闭（HTTP 403）：

```json
{ "code": 403, "msg": "AI推荐功能未开启" }
```

限流（HTTP 429）：

```json
{
  "code": 429,
  "msg": "AI推荐次数已达上限，请稍后再试",
  "data": {
    "limit": 5,
    "used": 5,
    "remaining": 0,
    "reset_after_sec": 3600,
    "retry_after_sec": 3600
  }
}
```

## 功能开关

`GET /app/features`（公开）

**响应 data：**

```json
{
  "ai_recommend": true,
  "catalog_recipe": true,
  "fridge": true,
  "blind_box": true
}
```

| 字段 | 配置项 | 说明 |
|------|--------|------|
| `ai_recommend` | `ai.recommend_enabled` | 关闭时 `/api/ai/*` 返回 403 |
| `catalog_recipe` | `ai.catalog_enabled`（默认随 recommend） | 关闭时 `/api/catalog-recipes/*` 返回 403 |
| `fridge` | `fridge.enabled`（默认 true） | 关闭时 `/api/fridge/*` 返回 403 |
| `blind_box` | `blind_box.enabled`（默认 true） | 关闭时 `/api/orders/blind-box/*` 返回 403 |

## AI 限流（独立计数）

| 场景 | 配置 | 默认 | 计次时机 |
|------|------|------|----------|
| AI 推荐 | `ai.rate_limit.recommend` | 2h / 5 次 | 每次 `POST /ai/recommend` 调 LLM 前 |
| 菜谱生成 | `ai.rate_limit.catalog` | 2h / 5 次 | `lookup` 需调 DeepSeek 时；纯查库不计次 |

## 菜谱 JSON 字段约定

`ingredients`、`seasonings`、`steps` 在 API 中为 **JSON 字符串**（与 DB 一致），不是嵌套对象。

**食材 / 调料元素：**

```json
{ "name": "番茄", "amount": "2个" }
```

**步骤：** 字符串数组，如 `["打蛋","合炒"]`。

---

变更记录：2026-06-12 初版
