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

# >>> 关键修改：替换为阿里云 Alpine 镜像源（支持 v3.20）
RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine/|https://mirrors.aliyun.com/alpine/|g' /etc/apk/repositories && \
    apk add --update --no-cache ${ALPINE_PKG_BASE} ${ALPINGE_PKG_EXTRA}

WORKDIR /app
COPY go.mod vendor* ./
RUN [ ! -d "vendor" ] && go mod download all || echo "skipping..."
COPY . .

ARG MAKE="make build-${TARGETARCH}"
RUN $MAKE


# ===================================
# 第二阶段：最终运行镜像 (Final Stage)
# ===================================
FROM alpine:3.20

ARG TARGETARCH
ARG VERSION

LABEL Name=app-demo-go Version=${VERSION}

RUN sed -i 's|https://dl-cdn.alpinelinux.org/alpine/|https://mirrors.aliyun.com/alpine/|g' /etc/apk/repositories && \
    apk add --update --no-cache ca-certificates dumb-init && \
    apk --no-cache upgrade

# 从 builder 阶段复制二进制文件
COPY --from=builder /app/app-demo-go-${TARGETARCH} /app-demo-go

# 复制配置
COPY --from=builder /app/res/ /res/

# 暴露端口
EXPOSE 59891

# 入口点应为通用名称（不带架构后缀）
ENTRYPOINT ["/app-demo-go"]
CMD ["-cp=keeper.http://edgex-core-keeper:59890", "--registry"]