# syntax=docker/dockerfile:1
# 固定工具链和依赖锁文件；镜像只带程序，运行owner/密钥/业务数据独立保存。
FROM golang@sha256:154bd7001b6eb339e88c964442c0ad6ed5e53f09844cc818a41ce4ecb3ce3b43 AS go-build
ENV GOWORK=off CGO_ENABLED=0 GOTOOLCHAIN=local
WORKDIR /src
COPY contracts/ ./contracts/
COPY packages/ ./packages/
COPY services/ ./services/
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    mkdir -p /out && \
    go -C services/account build -mod=readonly -o /out/account . && \
    go -C services/document build -mod=readonly -o /out/document . && \
    go -C services/search build -mod=readonly -o /out/search .

FROM node@sha256:a9f5f7c91a432850b2a8a7797adf5eadb6c733ceed61167806cee7ea7fbc29df AS application
ARG TARGETARCH
ENV NODE_ENV=production PLAYWRIGHT_BROWSERS_PATH=/opt/playwright
WORKDIR /app
COPY package.json package-lock.json ./
COPY services/collaboration/package.json ./services/collaboration/package.json
COPY web/package.json ./web/package.json
# Pandoc版本由现有转换器核验为3.8，不能换成系统仓库里的任意版本。
# 摘要来自jgm/pandoc官方3.8资产；构建失败时停止，不关闭完整性检查。
RUN test "$TARGETARCH" = "amd64" && \
    apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl openssl fonts-noto-cjk fonts-arphic-ukai fonts-liberation && \
    curl -fL --retry 3 https://github.com/jgm/pandoc/releases/download/3.8/pandoc-3.8-linux-amd64.tar.gz -o /tmp/pandoc.tar.gz && \
    echo '1a035c1f7d295c353f614461fa4bc13d703c0e865387d44341d1556d703de59a  /tmp/pandoc.tar.gz' | sha256sum -c - && \
    tar -xzf /tmp/pandoc.tar.gz -C /opt && \
    ln -s /opt/pandoc-3.8/bin/pandoc /usr/local/bin/pandoc && \
    rm /tmp/pandoc.tar.gz && \
    npm ci --include=dev && \
    npx --no-install playwright install --with-deps chromium && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /app/.runtime
COPY . .
RUN npm run typecheck --workspace web && npm run build --workspace web
COPY --from=go-build /out/ /opt/collab/bin/
# Compose必须启用init，监督器管理应用整组，转换子进程继续沿现有进程组取消。
CMD ["node", "scripts/server-start.mjs"]
