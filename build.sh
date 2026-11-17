#!/bin/bash
# 打包 8018 服务（./cmd/server）。
# 不传参数时按当前系统打包；也可指定目标系统与架构，例如：bash build.sh linux amd64
set -euo pipefail

cd "$(dirname "$0")"

target_os="${1:-$(go env GOOS)}"
target_arch="${2:-$(go env GOARCH)}"

suffix=""
if [ "$target_os" = "windows" ]; then
	suffix=".exe"
fi

output="dist/ctaf-fz-$(date +%Y%m%d%H%M%S)${suffix}"

mkdir -p dist

echo "目标系统：${target_os}/${target_arch}"
echo "输出文件：${output}"

if GOOS="$target_os" GOARCH="$target_arch" CGO_ENABLED=0 go build -o "$output" ./cmd/server; then
	echo "打包成功：${output}"
else
	echo "打包失败" >&2
	exit 1
fi
