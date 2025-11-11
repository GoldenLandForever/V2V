# Swagger API 文档

## 概述

本项目已集成 Swagger UI，用于可视化和交互式测试 API 接口。

## 启动项目

```bash
cd /media/xc/my/V2V
./V2V
```

或者如果需要重新构建：

```bash
cd /media/xc/my/V2V
go build
./V2V
```

## 访问 Swagger UI

启动服务后，打开浏览器访问：

```
http://localhost:8080/swagger/index.html
```

## API 端点

### V2T（视频转文字）

- **POST /V2T** - 提交视频转文字任务
- **GET /V2T/:task_id** - 获取 V2T 任务结果
- **POST /V2T/LoraText** - 更新任务 Lora 文本

### T2I（文字生成图片）

- **POST /T2I** - 提交文字生成图片任务

### I2V（图片生成视频）

- **POST /I2V** - 提交图片生成视频任务
- **GET /I2V/:task_id** - 获取 I2V 任务结果
- **POST /I2VCallback/:task_id** - I2V 任务回调处理

### FFmpeg

- **GET /FFmpeg/:task_id** - FFmpeg 处理器

### SSE（服务器推送事件）

- **GET /events** - 连接 SSE 事件流

## 功能说明

Swagger 文档提供了以下功能：

1. **API 查看** - 查看所有可用的 API 端点及其详细说明
2. **请求测试** - 直接在 UI 中测试 API 端点，无需第三方工具
3. **参数配置** - 查看和配置请求参数、请求体格式等
4. **响应展示** - 查看预期的响应格式和状态码

## 更新 Swagger 文档

如果修改了 API 端点或注释，需要重新生成 Swagger 文档：

```bash
cd /media/xc/my/V2V
/home/xc/go/lib/bin/swag init
```

或者使用别名（如果设置了）：

```bash
cd /media/xc/my/V2V
swag init
```

## 文档文件

- `docs/docs.go` - Go 代码形式的 Swagger 文档
- `docs/swagger.json` - JSON 格式的 Swagger 文档
- `docs/swagger.yaml` - YAML 格式的 Swagger 文档

## 注意事项

- 确保 RabbitMQ、Redis 等依赖服务正常运行
- API 文档中的所有请求示例都可以在 Swagger UI 中直接执行
- 生成的文档文件应该被纳入版本控制中

## 相关链接

- [Swag 文档](https://github.com/swaggo/swag)
- [Gin-Swagger](https://github.com/swaggo/gin-swagger)
- [Swagger 规范](https://swagger.io/)
