-- 冰箱食材：家庭库存与拍照识别任务

USE recipe_app;

CREATE TABLE IF NOT EXISTS fridge_items (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  family_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(100) NOT NULL,
  amount VARCHAR(50) DEFAULT '',
  expiry_date DATE NULL,
  note VARCHAR(200) DEFAULT '',
  source VARCHAR(20) NOT NULL COMMENT 'manual | photo',
  scan_id BIGINT UNSIGNED NULL,
  added_by BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at DATETIME NULL,
  INDEX idx_fridge_items_family (family_id),
  INDEX idx_fridge_items_expiry (family_id, expiry_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS fridge_scans (
  id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  family_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  task_id VARCHAR(36) NOT NULL,
  image_key VARCHAR(200) NOT NULL,
  image_url VARCHAR(500) NOT NULL,
  status VARCHAR(20) NOT NULL COMMENT 'pending|processing|done|failed|confirmed',
  recognized_items JSON NULL,
  error_msg TEXT,
  confirmed_at DATETIME NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uk_fridge_scans_task (task_id),
  INDEX idx_fridge_scans_family (family_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
