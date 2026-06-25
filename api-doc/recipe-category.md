# 家庭菜谱与分类

## GET /categories/public

公开菜谱中出现过的分类名（公开，无需登录）。

**响应 data：** `["家常菜","荤菜",...]`（字符串数组）

---

## GET /categories

当前家庭的菜谱分类列表（需登录）。会先同步 `recipes` 表中的分类。

**响应 data：** 分类对象数组 `[{ "id", "name", "sort_order", ... }]`

---

## GET /recipes

菜谱列表（可选登录）。

- 已登录：本家庭全部 + 其他家庭 `is_public=true` 的公开菜谱
- 未登录：仅公开菜谱

**Query：**

| 参数 | 说明 |
|------|------|
| keyword | 菜名模糊搜索 |
| category | 分类筛选 |
| page | 页码，默认 1 |
| page_size | 每页条数，默认 20 |

**响应 data：**

```json
{
  "list": [ { "id", "name", "category", "cover_url", "creator", ... } ],
  "total": 100,
  "page": 1,
  "page_size": 20,
  "has_more": false
}
```

---

## GET /recipes/:id

菜谱详情（可选登录）。本家庭或公开可见。已登录时响应含 `is_favorited`。

---

## POST /recipes

创建家庭菜谱（需登录）。`creator_id`、`family_id` 由服务端注入。

**Body：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 菜名 |
| category | string | 否 | 分类，默认「其他」；不存在则自动创建 |
| ingredients | string | 否 | JSON 字符串，食材数组 |
| seasonings | string | 否 | JSON 字符串，调料数组 |
| steps | string | 否 | JSON 字符串，步骤数组 |
| cook_time | int | 否 | 分钟 |
| difficulty | string | 否 | `easy` / `medium` / `hard` |
| cover_url | string | 否 | 封面 URL |
| image_key | string | 否 | OSS key |
| tips | string | 否 | 小贴士 |
| is_public | bool | 否 | 不传时默认 `true` |

**示例：**

```json
{
  "name": "番茄炒蛋",
  "category": "家常菜",
  "ingredients": "[{\"name\":\"番茄\",\"amount\":\"2个\"},{\"name\":\"鸡蛋\",\"amount\":\"3个\"}]",
  "seasonings": "[{\"name\":\"盐\",\"amount\":\"适量\"},{\"name\":\"糖\",\"amount\":\"少许\"}]",
  "steps": "[\"打蛋\",\"番茄切块\",\"合炒出锅\"]",
  "cook_time": 15,
  "difficulty": "easy",
  "tips": "先炒蛋更嫩"
}
```

**响应 data：** 完整菜谱对象（含新 `id`）。

**错误：** 400 请先加入家庭

---

## PUT /recipes/:id

更新菜谱（需登录）。仅更新非零值字段；须属于当前家庭。

---

## DELETE /recipes/:id

软删除菜谱（需登录）。仅创建者可删。

---

## POST /recipes/:id/cooked

烹饪次数 +1（需登录）。须属于当前家庭。

---

变更记录：2026-06-12 初版（含 seasonings 字段说明）
