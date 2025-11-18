# V2V 项目 Swagger 集成指南

## 📋 概述

已成功为 V2V 项目集成了 Swagger UI，用于可视化和交互式测试所有 API 接口。

## 🚀 快速启动

### 1. 启动服务

```bash
cd /media/xc/my/V2V

# 编译（如果是第一次或修改了代码）
go build

# 运行服务
./V2V
```

服务将在 `http://localhost:8080` 启动。

### 2. 访问 Swagger UI

打开浏览器，访问：

```
http://localhost:8080/swagger/index.html
```

## 📚 API 文档

### V2T（视频转文字）

| 方法 | 端点 | 描述 |
|------|------|------|
| POST | `/V2T` | 提交视频转文字任务 |
| GET | `/V2T/:task_id` | 获取 V2T 任务结果 |
| POST | `/V2T/LoraText` | 更新任务 Lora 文本 |

**示例请求（V2T）：**

```json
POST /V2T
{
  "video_url": "https://example.com/video.mp4",
  "user_id": 1
}
```

### T2I（文字生成图片）

| 方法 | 端点 | 描述 |
|------|------|------|
| POST | `/T2I` | 提交文字生成图片任务 |

**示例请求（T2I）：**

```json
POST /T2I
{
  "user_id": 1,
  "text": "A beautiful sunset over mountains",
  "priority": 5
}
```

### I2V（图片生成视频）

| 方法 | 端点 | 描述 |
|------|------|------|
| POST | `/I2V` | 提交图片生成视频任务 |
| GET | `/I2V/:task_id` | 获取 I2V 任务结果 |
| POST | `/I2VCallback/:task_id` | 处理 I2V 任务回调 |

**示例请求（I2V）：**

```json
POST /I2V
{
  "task_id": 123456,
  "user_id": 1
}
```

### FFmpeg

| 方法 | 端点 | 描述 |
|------|------|------|
| GET | `/FFmpeg/:task_id` | FFmpeg 处理器 |

### 其他

| 方法 | 端点 | 描述 |
|------|------|------|
| GET | `/events` | SSE 事件流 |
| GET | `/debug/pprof/*` | 性能分析端点 |

## 🛠️ 使用工具

### 在 Swagger UI 中测试

1. 在浏览器中访问 `http://localhost:8080/swagger/index.html`
2. 选择要测试的 API 端点
3. 点击 "Try it out" 按钮
4. 输入必要的参数
5. 点击 "Execute" 发送请求
6. 查看响应结果

### 使用 Postman

导入 `postman_collection.json` 文件：

1. 打开 Postman
2. 选择 "Import" 或 "File" → "Import"
3. 选择 `postman_collection.json` 文件
4. 设置 `base_url` 变量为 `http://localhost:8080`
5. 开始测试 API

### 使用 curl 命令

```bash
# 提交 T2I 任务
curl -X POST "http://localhost:8080/T2I" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "text": "A beautiful sunset",
    "priority": 5
  }'

# 获取任务结果
curl -X GET "http://localhost:8080/V2T/123456"
```

## 🔄 更新文档

如果修改了 API 端点或注释，需要重新生成 Swagger 文档：

```bash
cd /media/xc/my/V2V

# 使用 swag 工具生成文档
/home/xc/go/lib/bin/swag init

# 或设置别名后使用
swag init

# 然后重新编译和运行
go build
./V2V
```

## 📁 生成的文件

项目新增文件和目录：

```
V2V/
├── docs/
│   ├── docs.go          # Go 形式的 Swagger 文档
│   ├── swagger.json     # JSON 格式的 Swagger 文档
│   └── swagger.yaml     # YAML 格式的 Swagger 文档
├── SWAGGER.md           # Swagger 使用说明
├── QUICKSTART.md        # 快速启动指南（本文件）
├── postman_collection.json # Postman API 集合
├── test_api.sh          # API 测试脚本
└── main.go              # 已更新，添加了 Swagger 路由

更新的文件：
├── controller/T2I.go    # 添加了 Swagger 注释
├── controller/V2T.go    # 添加了 Swagger 注释
├── controller/I2V.go    # 添加了 Swagger 注释
├── controller/FFmpeg.go # 添加了 Swagger 注释
└── go.mod               # 添加了 swag 相关依赖
```

## ⚙️ 依赖要求

项目需要以下依赖已正确安装：

- `github.com/swaggo/swag` - Swagger 生成工具
- `github.com/swaggo/gin-swagger` - Gin 的 Swagger 中间件
- `github.com/swaggo/files` - Swagger UI 文件

这些依赖已在 `go.mod` 中添加。

## 🐛 常见问题

### 问：访问 Swagger UI 时出现 404 错误

**答：** 
1. 确保服务已启动在 `http://localhost:8080`
2. 确保使用了正确的 URL：`http://localhost:8080/swagger/index.html`
3. 检查 docs 文件夹中的文件是否存在

### 问：修改了 API 但文档没有更新

**答：** 需要运行 `swag init` 重新生成文档，然后重新编译和运行服务

### 问：出现 "import cycle not allowed" 错误

**答：** 确保 `docs` 文件夹存在且 `docs.go` 中正确导入了 docs 包

## 📞 支持

如有问题，请检查：

1. 所有依赖是否已正确安装
2. docs 文件夹及其中的文件是否存在
3. main.go 中是否正确导入了 docs 包
4. 是否运行了 `swag init` 命令

## 📖 参考资源

- [Swag GitHub](https://github.com/swaggo/swag)
- [Gin-Swagger GitHub](https://github.com/swaggo/gin-swagger)
- [Swagger 官方文档](https://swagger.io/)
- [OpenAPI 规范](https://spec.openapis.org/oas/v2.0)

---

**项目已成功集成 Swagger UI！现在可以通过 Web 界面查看和测试所有 API 接口了。** ✨

## 💰 Token 系统（用户余额管理）

V2V 项目包含了 Token 系统，用于管理用户在各项任务（T2I、I2V、V2T 等）中的消耗配额。

### 数据库迁移

在数据库中执行以下 SQL 语句来创建 Token 相关的表：

```sql
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
```

### Token API 接口

#### 1. 扣除 Token（T2I 任务）

**POST** `/api/v1/token/deduct-t2i`

**幂等性保证**：通过检查 `t2i_tasks` 表中的任务状态实现。仅当任务状态为 `pending` 时才会扣除 Token。

**请求体：**
```json
{
  "task_id": 123456789,
  "user_id": 987654321,
  "token_count": 10
}
```

**成功响应 (200)：**
```json
{
  "success": true,
  "message": "token deducted successfully",
  "remaining_tokens": 90
}
```

#### 2. 添加 Token

**POST** `/api/v1/token/add/:user_id/:amount`

为用户添加 Token（充值、奖励等）。

**示例：**
```
POST /api/v1/token/add/987654321/100
```

#### 3. 查询 Token 信息

**GET** `/api/v1/token/info/:user_id`

获取用户的 Token 余额和 VIP 等级。

#### 4. 初始化用户 Token

**POST** `/api/v1/token/init/:user_id`

为新注册用户初始化 Token（初始值：100）。
