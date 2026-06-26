# 冰箱食材

> 开关：`fridge.enabled`（默认 `true`）；`GET /app/features` → `fridge`  
> 拍照识别依赖 `image_worker.enabled` 与树莓派 WebSocket 在线。

## GET /fridge/items

家庭冰箱库存列表（需登录）。

**排序**：有保质期的按日期升序，无保质期在后；同组按更新时间降序。

**响应 data：** `FridgeItem` 数组

```json
[
  {
    "id": 1,
    "family_id": 1,
    "name": "牛奶",
    "amount": "1盒",
    "expiry_date": "2026-07-01",
    "note": "",
    "source": "manual",
    "added_by": 1,
    "created_at": "...",
    "updated_at": "..."
  }
]
```

---

## POST /fridge/items

手动新增食材（需登录）。支持单条或批量。

**Body（单条）：**

```json
{
  "name": "鸡蛋",
  "amount": "6个",
  "expiry_date": "2026-06-20",
  "note": "冷藏"
}
```

**Body（批量）：**

```json
{
  "items": [
    { "name": "番茄", "amount": "3个" },
    { "name": "豆腐", "amount": "1块", "expiry_date": "2026-06-18" }
  ]
}
```

**响应 data：** 单条返回对象；批量返回数组。

**错误：** 400 请先加入家庭 / 食材名称不能为空

---

## PUT /fridge/items/:id

更新食材（需登录）。仅更新非空字段。

---

## DELETE /fridge/items/:id

删除食材（软删除，需登录）。

---

## POST /fridge/scans

拍照上传并发起识别（需登录）。

**请求：** `multipart/form-data`，字段 `file`（图片，启用内容安全时 ≤1MB）

**流程：**

1. 调用微信 **img_sec_check** 检测图片
2. 上传 OSS
3. 创建 `fridge_scans` 任务
4. 经 WebSocket 派发 `recognize` 任务给树莓派（`meta.scope=fridge`）
5. 返回 `scan_id` 供轮询

图片违规时 HTTP 400：`您发布的内容含违规信息，请修改后重试`

**响应 data（成功）：**

```json
{
  "id": 1,
  "status": "processing",
  "task_id": "uuid",
  "image_url": "https://..."
}
```

**响应（网关离线，HTTP 503）：**

```json
{
  "code": 503,
  "msg": "图片识别服务离线",
  "data": {
    "scan": { "id": 1, "status": "failed", ... },
    "worker_offline": true
  }
}
```

---

## GET /fridge/scans/:id

轮询识别任务（需登录）。建议间隔 1–2 秒。

**响应 data：**

```json
{
  "id": 1,
  "status": "done",
  "image_url": "https://...",
  "error_msg": "",
  "recognized_items": [
    { "name": "黄瓜", "amount": "2根" },
    { "name": "鸡蛋", "amount": "" }
  ],
  "confirmed_at": null
}
```

| status | 说明 |
|--------|------|
| pending / processing | 识别中 |
| done | 可确认入库 |
| failed | 识别失败，见 error_msg |
| confirmed | 已确认写入库存 |

`recognized_items` 无识别结果时为 `[]`，不会是 `null`。

---

## POST /fridge/scans/:id/retry

对卡在 `processing` 或 `failed` 的识别任务重新派发（需登录）。使用原 `task_id` 与 OSS 图片，不重新上传。

**前置条件：** 树莓派 ImageWorker WebSocket 在线。

**响应 data：** 更新后的 `FridgeScan`（`status=processing`）

**错误：**

| HTTP | 说明 |
|------|------|
| 400 | 任务不可重试（如已 `done` / `confirmed`） |
| 404 | 扫描记录不存在 |
| 503 | 图片识别服务离线 |

---

## POST /fridge/scans/:id/confirm

用户勾选/编辑后确认入库（需登录）。仅 `status=done` 且未确认时可调用。

**Body：**

```json
{
  "items": [
    { "name": "黄瓜", "amount": "2根", "expiry_date": "2026-06-25", "note": "" },
    { "name": "鸡蛋", "amount": "6个" }
  ]
}
```

**响应 data：** 新创建的 `FridgeItem` 数组（`source=photo`，含 `scan_id`）

**错误：** 400 识别任务不可确认 / 请至少选择一条食材

---

## 树莓派 ImageWorker 协议（冰箱识别）

**下发任务：**

```json
{
  "type": "task",
  "task_id": "<与 fridge_scans.task_id 一致>",
  "action": "recognize",
  "oss_key": "fridge/xxx.jpg",
  "oss_url": "https://...",
  "meta": { "scope": "fridge", "scan_id": 1 }
}
```

**回传 task_result（成功）：**

```json
{
  "type": "task_result",
  "task_id": "<同上>",
  "status": "ok",
  "action": "recognize",
  "meta": { "scope": "fridge", "scan_id": 1 },
  "detail": {
    "items": [
      { "name": "鸡蛋", "amount": "6个" },
      { "name": "番茄", "amount": "3个" }
    ]
  }
}
```

**兼容旧格式 detail：**

```json
{ "ingredients": ["鸡蛋", "番茄"] }
```

服务端会映射为仅含 `name` 的候选列表，用户在确认页补全数量与保质期。

**失败：**

```json
{
  "type": "task_result",
  "status": "error",
  "action": "recognize",
  "error_msg": "识别失败原因",
  "meta": { "scope": "fridge", "scan_id": 1 }
}
```

---

变更记录：2026-06-24 拍照上传增加 img_sec_check；2026-06-12 初版；2026-06-16 补充 `/retry` 与空识别结果约定
