# BENZHI_README

基于 Go 实现的岩芯不可逆取样保全服务 HTTP API 项目，一款后端服务，岩芯不可逆取样保全服务提供岩芯约束建档、取样申请预检、复核退回、授权执行、余样核验、来源冻结和凭据验真。

## 项目说明
- 项目：benzhi-project-8320a7d1-1494-4b03-afe5-0554e4635597
- 项目用途：岩芯不可逆取样保全服务提供岩芯约束建档、取样申请预检、复核退回、授权执行、余样核验、来源冻结和凭据验真。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -addr=127.0.0.1:19087 -selfcheck
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-8320a7d1-1494-4b03-afe5-0554e4635597-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-8320a7d1-1494-4b03-afe5-0554e4635597-arm64 linux/arm64
docker run -it benzhi-project-8320a7d1-1494-4b03-afe5-0554e4635597-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -addr=127.0.0.1:19087 -selfcheck`
