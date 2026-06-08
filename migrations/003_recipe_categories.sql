-- 家庭菜谱分类表（从 recipes.category 同步，AI 推荐新分类时自动写入）

CREATE TABLE IF NOT EXISTS recipe_categories (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  family_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(50) NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uk_family_category_name (family_id, name),
  INDEX idx_family (family_id),
  FOREIGN KEY (family_id) REFERENCES families(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
