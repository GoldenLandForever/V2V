-- Migration: create t_v2t_tasks table
CREATE TABLE IF NOT EXISTS `t_v2t_tasks` (
  `task_id` BIGINT NOT NULL,
  `user_id` BIGINT NOT NULL,
  `status` VARCHAR(32) NOT NULL,
  `result` text COLLATE utf8mb4_unicode_ci,
  `token` int unsigned DEFAULT NULL COMMENT 'token',
  `video_url` VARCHAR(1024),
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`task_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
