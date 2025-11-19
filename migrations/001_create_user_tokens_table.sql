-- 用户Token表 - 用于管理用户的token余额和VIP等级
CREATE TABLE IF NOT EXISTS `t_user_tokens` (
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `tokens` bigint unsigned NOT NULL DEFAULT 0 COMMENT 'Token余额',
  `vip_level` tinyint unsigned NOT NULL DEFAULT 0 COMMENT 'VIP等级: 0-普通用户, 1-VIP1, 2-VIP2, 3-VIP3',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`user_id`),
  KEY `idx_vip_level` (`vip_level`),
  KEY `idx_updated_at` (`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户Token余额表';

-- 如果需要查看用户Token使用日志，可创建此表（可选）
CREATE TABLE IF NOT EXISTS `t_token_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '日志ID',
  `user_id` bigint unsigned NOT NULL COMMENT '用户ID',
  `task_id` bigint unsigned NOT NULL COMMENT '任务ID',
  `task_type` varchar(32) NOT NULL COMMENT '任务类型: T2I, I2V, V2T等',
  `token_change` bigint NOT NULL COMMENT 'Token变化量（负数表示扣除）',
  `operation_type` varchar(32) NOT NULL DEFAULT 'deduct' COMMENT '操作类型: deduct-扣除, refund-退款, add-充值',
  `status` varchar(32) NOT NULL DEFAULT 'success' COMMENT '操作状态: success, failed, cancelled',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_task_id` (`task_id`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Token操作日志表';


CREATE TABLE `user` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` bigint NOT NULL,
  `username` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL,
  `password` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL,
  `email` varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci DEFAULT NULL,
  `gender` tinyint NOT NULL DEFAULT '0',
  `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `idx_username` (`username`) USING BTREE,
  UNIQUE KEY `idx_user_id` (`user_id`) USING BTREE
) ENGINE=InnoDB AUTO_INCREMENT=8 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci ROW_FORMAT=DYNAMIC

CREATE TABLE `t2i_tasks` (
  `task_id` bigint unsigned NOT NULL COMMENT ' IDID',
  `user_id` bigint unsigned NOT NULL COMMENT ' ID',
  `status` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT 'pending/processing/completed/failed/canceled',
  `token` int unsigned DEFAULT NULL COMMENT 'token',
  `image_url` text COLLATE utf8mb4_unicode_ci COMMENT 'URL',
  `prompt` text COLLATE utf8mb4_unicode_ci,
  `error_message` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`task_id`),
  KEY `idx_t2i_user` (`user_id`),
  KEY `idx_t2i_status` (`status`),
  KEY `idx_t2i_created` (`created_at`),
  KEY `idx_t2i_user_status` (`user_id`,`status`),
  KEY `idx_t2i_status_created` (`status`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci

CREATE TABLE `i2v_task_main` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `task_id` bigint unsigned NOT NULL COMMENT ' taskID',
  `user_id` bigint unsigned NOT NULL COMMENT ' useridID',
  `status` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `video_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'ID',
  `video_url` varchar(1024) COLLATE utf8mb4_unicode_ci COMMENT 'URL',
  `index` int unsigned NOT NULL COMMENT ' index',
  `token` int unsigned DEFAULT NULL COMMENT 'token',
  `prompt` text COLLATE utf8mb4_unicode_ci,
  `error_message` text COLLATE utf8mb4_unicode_ci,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_i2v_task` (`task_id`),
  KEY `idx_i2v_user` (`user_id`),
  KEY `idx_i2v_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci

CREATE TABLE `i2v_video_details` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `task_id` bigint unsigned NOT NULL COMMENT ' ID',
  `video_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'ID',
  `video_url` text COLLATE utf8mb4_unicode_ci COMMENT 'URL',
  `token_cost` int unsigned DEFAULT NULL COMMENT 'token',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_i2v_videoid` (`video_id`),
  UNIQUE KEY `ux_i2v_task_video` (`task_id`,`video_id`),
  KEY `idx_i2v_task` (`task_id`),
  KEY `idx_i2v_videoid` (`video_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci