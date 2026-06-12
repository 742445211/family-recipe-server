-- 全局菜谱库：跨家庭共享，支持同一菜名多种做法

USE recipe_app;

CREATE TABLE IF NOT EXISTS catalog_recipes (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(200) NOT NULL COMMENT '展示菜名',
  name_key VARCHAR(200) NOT NULL COMMENT '规范化菜名，精确检索',
  variant_label VARCHAR(100) NOT NULL DEFAULT '经典做法' COMMENT '做法标签',
  is_default TINYINT(1) NOT NULL DEFAULT 0 COMMENT '该菜名下默认做法',
  category VARCHAR(50) NOT NULL DEFAULT '其他',
  ingredients JSON,
  seasonings JSON,
  steps JSON,
  cook_time INT UNSIGNED NOT NULL DEFAULT 0,
  difficulty ENUM('easy','medium','hard') NOT NULL DEFAULT 'medium',
  cover_url VARCHAR(500) DEFAULT '',
  tips TEXT,
  source VARCHAR(32) NOT NULL COMMENT 'ai_search | ai_recommend',
  content_hash CHAR(64) NOT NULL COMMENT '内容去重哈希',
  use_count INT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_catalog_name_key (name_key),
  INDEX idx_catalog_content_hash (content_hash),
  UNIQUE KEY uk_catalog_name_hash (name_key, content_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
