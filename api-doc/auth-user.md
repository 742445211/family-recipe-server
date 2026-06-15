# 认证与用户

## POST /auth/login

微信登录（公开）。

**Body：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| code | string | 是 | `wx.login()` 返回的 code |
| nickname | string | 否 | 昵称（新用户/更新） |
| avatar_url | string | 否 | 头像 URL |

**响应 data：**

```json
{
  "token": "eyJ...",
  "user_id": 1,
  "openid": "oXXXX",
  "nickname": "昵称",
  "avatar": "https://..."
}
```

---

## GET /users/me

当前用户信息（需登录）。

**响应 data：**

```json
{
  "id": 1,
  "openid": "oXXXX",
  "nickname": "昵称",
  "avatar_url": "https://...",
  "current_family_id": 1,
  "is_chef": true
}
```

---

## PUT /users/me

更新用户信息（需登录）。仅非空字段生效。

**Body：**

| 字段 | 类型 | 说明 |
|------|------|------|
| nickname | string | 昵称 |
| avatar_url | string | 头像 |
| current_family_id | uint64 | 切换当前家庭 |

**响应：** `{ "code": 0, "msg": "ok" }`

---

变更记录：2026-06-12 初版
