#!/bin/bash

# 获取JWT token
TOKEN=$(go run cmd/test-token/main.go -secret "your-jwt-secret")
if [ $? -ne 0 ]; then
    echo "Failed to generate JWT token"
    exit 1
fi

# 设置环境变量
export TEST_TOKEN="$TOKEN"
export TEST_CONCURRENT=10000
export TEST_TOTAL=100000

# 运行负载测试
go test -v -run TestLoad -timeout 30m internal/api/load_test.go

# 输出结果
echo "Load test completed"
