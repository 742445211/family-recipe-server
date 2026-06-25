# 通知、上传与 WebSocket

## GET /notifications/unread

当前用户未读通知（需登录）。

**响应 data：** 通知数组，每项含 `order_date`、`meal_type` 便于跳转。

---

## POST /notifications/:id/read

标记通知已读（需登录）。

---

## GET /notification-channels

用户通知通道配置列表（需登录）。secret 以脱敏形式返回。

---

## POST /notification-channels

创建通知通道（需登录）。

**Body：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| channel | string | 是 | 通道类型枚举 |
| enabled | bool | 否 | |
| endpoint | string | 否 | |
| secret | string | 否 | |
| topic | string | 否 | |

**channel 枚举：** `websocket`、`wechat_subscribe`、`wecom_workbench`、`server_chan`、`bark`、`ntfy`

---

## PUT /notification-channels/:id

更新通道（需登录）。

---

## DELETE /notification-channels/:id

删除通道（需登录）。

---

## POST /upload

上传图片（需登录）。支持 jpg/jpeg/png/webp/gif，最大 10MB。

**请求：** `multipart/form-data`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | file | 是 | 图片文件 |
| recipe_id | uint64 | 否 | 关联本家庭菜谱 ID（压缩完成后更新封面） |

**错误：** 400 格式/大小不合法；菜谱不存在或非本家庭

---

## GET /api/ws

WebSocket 厨师通知（非 REST JSON）。

**鉴权：** query `token=<JWT>` 或 Header `Authorization: Bearer <JWT>`

**Origin：** 须匹配请求 Host 或 `server.allowed_origins` 配置

- 路径可由 `notification.websocket.path` 配置，默认 `/api/ws`
- Nginx 需配置 WebSocket Upgrade
- 在线时推送未读通知；点菜成功也会推送

---

## GET /uploads/*

静态文件（公开）。上传图片的本地/OSS 映射路径，见 `main.go` 静态目录配置。

---

变更记录：
- 2026-06-24 上传校验格式/大小与 recipe_id 归属；WebSocket 支持 Bearer 鉴权与 Origin 限制
- 2026-06-12 初版
