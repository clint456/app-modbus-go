# ===================================
# 第一阶段：构建阶段 (Builder Stage)
# ===================================
ARG BASE=golang:1.25-alpine
FROM ${BASE} AS builder

# 使用国内 Go 模块代理加速依赖下载
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=off
ENV EDGEX_SECURITY_SECRET_STORE="false"

ARG ALPINE_PKG_BASE="make git"
ARG ALPINE_PKG_EXTRA=""
ARG TARGETARCH
ARG TARGETOS=linux

# 使用阿里云 Alpine 镜像源加速
RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine/|https://mirrors.aliyun.com/alpine/|g' /etc/apk/repositories && \
    apk add --update --no-cache ${ALPINE_PKG_BASE} ${ALPINE_PKG_EXTRA}

# 直接把项目拷贝到 /app 目录（不设 WORKDIR）
COPY . /app/

# 如果有 vendor 就用，没有就下载依赖
RUN [ -d "/app/vendor" ] && echo "using vendor mode" || (cd /app && go mod download)

# 执行构建（强制进入 /app 目录执行 make）
RUN cd /app && make build-${TARGETARCH}

# ===================================
# 第二阶段：最终运行镜像 (Final Stage)
# ===================================
FROM alpine:3.20

ARG TARGETARCH
ARG VERSION

LABEL Name=app-demo-go Version=${VERSION}

# 使用阿里云源 + 安装必要工具
RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine/|https://mirrors.aliyun.com/alpine/|g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates dumb-init && \
    apk --no-cache upgrade

# 从 builder 复制二进制和配置（使用 builder 中的绝对路径）
COPY --from=builder /app/cmd/app-demo-go /app-demo-go
COPY --from=builder /app/cmd/res/        /res/

# 使用 dumb-init 作为 init 进程 + 运行程序
ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/app-demo-go", "-cp=keeper.http://edgex-core-keeper:59890", "--registry"]