# 家庭

## POST /families

创建家庭（需登录）。创建者成为 owner，并设为当前家庭。

**Body：**

| 字段 | 类型 | 必填 |
|------|------|------|
| name | string | 是 |

**响应 data：** 家庭对象（含 `id`、`name`、`invite_code`）及 **`token`**（已切换当前家庭的新 JWT，前端应替换本地 token）。

---

## POST /families/join

邀请码加入家庭（需登录）。已加入则幂等；成功后设为当前家庭。

**Body：**

| 字段 | 类型 | 必填 |
|------|------|------|
| invite_code | string | 是 |

**响应 data：** 家庭对象及 **`token`**（新 JWT）。

**错误：** 404 邀请码无效

---

## GET /families

当前用户所属家庭列表（需登录）。

---

## GET /families/:id/members

家庭成员列表（需登录，**须为该校家庭成员**）。

**错误：** 403 无权查看

**说明：** 响应中用户对象不含 `openid`，仅返回 `id`、`nickname`、`avatar_url`。

---

## POST /families/chef

切换当前用户在本家庭的厨师身份 `is_chef`（需登录）。

**响应 data：** `{ "is_chef": true/false }`

---

变更记录：
- 2026-06-24 创建/加入家庭返回新 token；成员列表需家庭归属校验并脱敏 openid
- 2026-06-12 初版
