#!/bin/bash

# V2V API 测试脚本

BASE_URL="http://localhost:8080"

echo "=== V2V API 测试脚本 ==="
echo ""

# 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}1. 测试 V2T 接口（提交视频转文字任务）${NC}"
echo "请确保已有有效的视频文件..."
echo ""

echo -e "${BLUE}2. 测试 T2I 接口（提交文字生成图片任务）${NC}"
echo "发送请求: POST /T2I"
curl -X POST "${BASE_URL}/T2I" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 1,
    "text": "A beautiful sunset over mountains",
    "priority": 5
  }' 2>/dev/null | json_pp
echo ""

echo -e "${BLUE}3. 访问 Swagger UI${NC}"
echo -e "${GREEN}URL: ${BASE_URL}/swagger/index.html${NC}"
echo ""

echo -e "${YELLOW}提示:${NC}"
echo "- 所有 API 端点都可以在 Swagger UI 中查看和测试"
echo "- 确保本地服务正在运行: ./V2V"
echo "- 确保 RabbitMQ 和 Redis 服务已启动"
echo ""
