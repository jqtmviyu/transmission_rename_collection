#!/bin/bash

# 创建 build 目录（如果不存在）
mkdir -p build

# 编译优化参数
LDFLAGS="-s -w"         # -s: 去掉符号表 -w: 去掉调试信息
EXTRA_FLAGS="-trimpath" # 移除编译路径信息

# 清理函数
clean() {
  local OS=$1
  local ARCH=$2
  local TARGET="build/rename_collection_${OS}_${ARCH}"

  if [ "$OS" = "windows" ]; then
    TARGET="${TARGET}.exe"
  fi

  if [ -f "$TARGET" ]; then
    echo "清理旧文件: $TARGET"
    rm "$TARGET"
  fi
}

# 编译函数
build() {
  local OS=$1
  local ARCH=$2
  local SUFFIX=""

  if [ "$OS" = "windows" ]; then
    SUFFIX=".exe"
  fi

  # 先清理旧文件
  clean $OS $ARCH

  echo "正在编译 $OS $ARCH 版本..."
  GOOS=$OS GOARCH=$ARCH go build \
    -ldflags="${LDFLAGS}" \
    ${EXTRA_FLAGS} \
    -o "build/rename_collection_${OS}_${ARCH}${SUFFIX}" \
    main.go

  # 为类Unix系统添加执行权限
  if [ "$OS" != "windows" ]; then
    chmod +x "build/rename_collection_${OS}_${ARCH}"
  fi
}

# 清理所有
clean_all() {
  echo "清理所有旧文件..."
  rm -f build/rename_collection_*
}

# 根据参数执行编译
case "$1" in
"linux-arm64")
  build linux arm64
  ;;
"linux-amd64")
  build linux amd64
  ;;
"windows-amd64")
  build windows amd64
  ;;
"darwin-amd64")
  build darwin amd64
  ;;
"all")
  clean_all
  build linux arm64
  build linux amd64
  build windows amd64
  build darwin amd64
  ;;
"clean")
  clean_all
  echo "清理完成！"
  exit 0
  ;;
*)
  echo "用法: $0 {linux-arm64|linux-amd64|windows-amd64|darwin-amd64|all|clean}"
  echo "示例:"
  echo "  $0 linux-arm64    # 仅编译 Linux ARM64 版本"
  echo "  $0 all            # 编译所有平台版本"
  echo "  $0 clean          # 清理所有编译文件"
  exit 1
  ;;
esac

echo "编译完成！"
