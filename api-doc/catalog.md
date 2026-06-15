# 全局菜谱库

> 开关：`catalog_recipe`（见 [overview.md](./overview.md)）  
> 限流：`catalog` 场景，默认 2h/5 次；纯查库不消耗配额。

## POST /catalog-recipes/lookup

按菜名精确查全局库；无记录或 `new_variant=true` 时 DeepSeek 生成并入库（需登录）。

**Body：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 菜名（精确匹配 `name_key`） |
| new_variant | bool | 否 | `true` 时生成同菜名另一种做法 |

**响应 data：**

```json
{
  "name": "番茄炒蛋",
  "generated": false,
  "selected_id": 1,
  "variants": [
    {
      "id": 1,
      "name": "番茄炒蛋",
      "name_key": "番茄炒蛋",
      "variant_label": "经典做法",
      "is_default": true,
      "category": "家常菜",
      "ingredients": "[...]",
      "seasonings": "[...]",
      "steps": "[...]",
      "cook_time": 15,
      "difficulty": "easy",
      "cover_url": "",
      "tips": "",
      "source": "family_import",
      "use_count": 3
    }
  ],
  "rate_limit": {
    "limit": 5,
    "used": 1,
    "remaining": 4,
    "reset_after_sec": 7200
  }
}
```

| 字段 | 说明 |
|------|------|
| generated | `true` 表示本次调用了 AI 并写入库 |
| variants | 同一菜名多种做法列表 |
| selected_id | 默认选中的 variant id |

**错误：**

- 403 功能未开启
- 429 菜谱生成次数已达上限
- 400 菜名为空

**前端典型流程：** lookup 预填表单 → 用户编辑 → `POST /recipes` 保存家庭菜谱。

---

## GET /catalog-recipes/:id

单条全局做法详情（需登录）。

---

## POST /catalog-recipes/:id/use

递增该做法的 `use_count`（需登录）。切换做法时可调用。

**响应：** `{ "code": 0, "msg": "ok" }`

---

## source 枚举

| 值 | 说明 |
|----|------|
| ai_search | 添加页搜索触发生成 |
| ai_recommend | AI 推荐批次写入 |
| family_import | 自家庭菜谱回填（运维脚本） |

---

变更记录：2026-06-12 初版
