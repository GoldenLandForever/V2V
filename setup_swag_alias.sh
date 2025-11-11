#!/bin/bash

# 设置 swag 命令别名和环境变量

# 检查 swag 是否存在
if [ ! -f "/home/xc/go/lib/bin/swag" ]; then
    echo "❌ 未找到 swag 命令"
    echo "请运行: go get -u github.com/swaggo/swag/cmd/swag@latest"
    exit 1
fi

# 添加别名到 .bashrc
if ! grep -q "alias swag=" ~/.bashrc; then
    echo "alias swag='/home/xc/go/lib/bin/swag'" >> ~/.bashrc
    echo "✅ 已为 .bashrc 添加 swag 别名"
else
    echo "✅ swag 别名已存在"
fi

# 添加别名到 .zshrc（如果使用 zsh）
if [ -f ~/.zshrc ]; then
    if ! grep -q "alias swag=" ~/.zshrc; then
        echo "alias swag='/home/xc/go/lib/bin/swag'" >> ~/.zshrc
        echo "✅ 已为 .zshrc 添加 swag 别名"
    fi
fi

# 设置 GOBIN 环境变量（可选）
if ! grep -q "export GOBIN=" ~/.bashrc; then
    echo "export GOBIN=/home/xc/go/lib/bin" >> ~/.bashrc
    echo "✅ 已为 .bashrc 添加 GOBIN 环境变量"
fi

echo ""
echo "请运行以下命令使设置生效:"
echo "source ~/.bashrc"
echo ""
echo "然后就可以在任何地方使用 'swag' 命令了"
