# GitHub Actions

仓库包含 CI 和 Release 两套工作流。

## CI

文件：`.github/workflows/ci.yml`

触发条件：

- 向 `main` 或 `master` 推送代码
- 创建或更新 Pull Request
- 在 Actions 页面手动触发

执行内容：

1. 检查所有 Go 文件是否经过 `gofmt`
2. 执行 `go vet ./...`
3. 执行 `go test ./...` 并生成覆盖率文件
4. 构建 Linux amd64 单二进制
5. 验证 Docker 镜像可以成功构建

覆盖率文件作为 Actions Artifact 保留 7 天。

## Release

文件：`.github/workflows/release.yml`

推送 `v*` 标签后自动：

1. 重新执行 `go vet` 和全量测试
2. 构建 Linux amd64/arm64、macOS amd64/arm64、Windows amd64
3. 将二进制、`README.md`、`docs/` 和 `.env.example` 打包
4. 生成 SHA-256 校验文件
5. 创建 GitHub Release 并上传产物
6. 构建 Linux amd64/arm64 镜像并推送到 GHCR

发布示例：

```bash
git tag v0.1.0
git push origin v0.1.0
```

镜像地址：

```text
ghcr.io/<owner>/<repository>:latest
ghcr.io/<owner>/<repository>:0.1.0
```

在 Actions 页面手动运行 Release 时，只生成临时构建产物，不创建正式 Release，也不推送容器镜像。

## 权限

工作流使用仓库自带的 `GITHUB_TOKEN`，不需要额外配置访问密钥。若组织策略限制发布，请在仓库以下位置确认写权限：

```text
Settings → Actions → General → Workflow permissions
```

Release 工作流需要：

- `contents: write`：创建 GitHub Release
- `packages: write`：向 GHCR 推送镜像

## 本地执行同等检查

```bash
unformatted="$(gofmt -l .)"; test -z "$unformatted"
go vet ./...
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o resource-center-linux .
docker build -t resource-center:local .
```
