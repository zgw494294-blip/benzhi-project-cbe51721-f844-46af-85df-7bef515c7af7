# BENZHI_README

## 项目说明
- 项目：benzhi-project-cbe51721-f844-46af-85df-7bef515c7af7
- 项目用途：TillSeal is a standard-library Go CLI for cash-drawer reconciliation with versioned atomic JSON persistence, immutable receipts, deterministic inspection, focused tests, documentation, and a bounded smoke workflow. Acceptance tests, smoke, vet, and formatting checks pass.
- Go 工具链：`golang:1.22.0`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/tillseal smoke
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-cbe51721-f844-46af-85df-7bef515c7af7-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-cbe51721-f844-46af-85df-7bef515c7af7-arm64 linux/arm64
docker run -it benzhi-project-cbe51721-f844-46af-85df-7bef515c7af7-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/tillseal smoke`
