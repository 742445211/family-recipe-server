# 点菜与收藏

## GET /orders

点菜列表（需登录）。

**Query：**

| 参数 | 说明 |
|------|------|
| date | `YYYY-MM-DD`，可选 |
| meal_type | `breakfast` / `lunch` / `dinner` / `supper`，可选 |

**响应 data：** 点菜数组，含预加载的 `recipe`、`adder`。

---

## POST /orders

点一道菜（需登录）。同家庭、同日期、同餐次、同菜谱不可重复。

**Body：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| recipe_id | uint64 | 是 | 菜谱 ID |
| date | string | 否 | 默认今天 |
| meal_type | string | 否 | 默认 `dinner` |
| quantity | int | 否 | 默认 1 |
| note | string | 否 | 备注 |

成功后异步通知厨师。

**错误：** 400 该餐次已点过这道菜 / 菜谱不存在或不属于当前家庭

---

## DELETE /orders/:id

取消点菜（需登录，软删除）。

---

## POST /orders/share

创建微信动态消息 `activity_id`（需登录）。

**响应 data：** `{ "activity_id": "..." }`

---

## GET /favorites

收藏列表（需登录）。

**Query：**

| 参数 | 说明 |
|------|------|
| page | 页码，默认 1 |
| page_size | 每页条数，默认 20，最大 50 |

**响应 data：**

```json
{
  "list": [ { "id", "recipe_id", "recipe": { ... } } ],
  "total": 10,
  "page": 1,
  "page_size": 20,
  "has_more": false
}
```

## POST /favorites/:id

收藏菜谱（需登录）。`:id` 为菜谱 ID。

---

## DELETE /favorites/:id

取消收藏（需登录）。

---

变更记录：2026-06-12 初版
