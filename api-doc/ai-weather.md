# AI 推荐与天气

> AI 接口开关：`ai_recommend`；关闭时 `/api/ai/*` 返回 403。  
> 推荐限流：`recommend` 场景，默认 2h/5 次。

## POST /ai/recommend

生成 AI 推荐批次（需登录）。

**Query：**

| 参数 | 说明 |
|------|------|
| meal_type | 可选，覆盖餐次：`breakfast`/`lunch`/`dinner`/`supper` 或中文 |

**响应 data：**

```json
{
  "batch_id": "uuid",
  "items": [
    {
      "item_id": "uuid",
      "name": "菜名",
      "meal_type": "lunch",
      "reason": "推荐理由",
      "existing_recipe_id": null
    }
  ],
  "rate_limit": { "limit": 5, "used": 1, "remaining": 4, "reset_after_sec": 7200 }
}
```

| 字段 | 说明 |
|------|------|
| existing_recipe_id | 非空表示家庭已有同名菜 |

**错误：** 429 AI 推荐次数已达上限

---

## GET /ai/items/:item_id

AI 推荐草稿详情（Redis，需登录）。

**响应 data：**

```json
{
  "item_id": "uuid",
  "batch_id": "uuid",
  "family_id": 1,
  "name": "菜名",
  "category": "家常菜",
  "meal_type": "lunch",
  "difficulty": "easy",
  "cook_time": 15,
  "ingredients": "[...]",
  "seasonings": "[...]",
  "steps": "[...]",
  "tips": "",
  "reason": "",
  "existing_recipe_id": null
}
```

---

## POST /ai/items/:item_id/import-recipe

将 AI 草稿导入家庭菜谱库（需登录）。

**响应 data：** 新建的家庭 `Recipe` 对象。

**错误：** 400 该菜已在家庭菜谱库中

---

## POST /ai/items/:item_id/add-order

从 AI 草稿导入（如需）并点菜（需登录）。

**Body：**

| 字段 | 类型 | 说明 |
|------|------|------|
| meal_type | string | 餐次 |
| date | string | 日期 |
| note | string | 备注 |
| quantity | int | 份数 |

**响应 data：** `DailyOrder` 对象。

---

## GET /weather

天气快照（公开，不受 AI 开关影响）。默认成都，Open-Meteo，缓存 3h。

**响应 data：** 天气对象（温度、描述等，见 `WeatherService`）。

---

变更记录：2026-06-12 初版
