#!/bin/bash
set -e

# 设置目标操作系统和架构
export GOOS=linux
export GOARCH=amd64

# 定义源文件和输出文件名
SOURCE_FILE="pkg/proxy/examples/main.go"
OUTPUT_NAME="proxy-example-${GOOS}-${GOARCH}"

echo "正在为 ${GOOS}/${GOARCH} 交叉编译 ${SOURCE_FILE}..."

# 执行编译命令
go build -o ${OUTPUT_NAME} ${SOURCE_FILE}

# 检查编译是否成功
if [ $? -eq 0 ]; then
  echo "编译成功！ 可执行文件: ${OUTPUT_NAME}"
  # 添加执行权限
  chmod +x ${OUTPUT_NAME}
  echo "已添加执行权限"
else
  echo "编译失败！"
  exit 1
fi 