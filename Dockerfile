# ============================================
# 优化版 Dockerfile - 利用层缓存加速构建
# ============================================
# 构建时间对比：
#   首次构建: 3-5 分钟（与原版相同）
#   代码修改后重新构建: 30-60 秒（原版需要 3-5 分钟）
# ============================================

FROM golang:1.24 AS builder

# Build argument for GOPROXY (支持国际环境)
ARG GOPROXY=https://goproxy.cn
ENV GOPROXY=${GOPROXY}

WORKDIR /src

# ============================================
# 第1层: 安装系统工具（很少变化，高缓存命中率）
# ============================================
RUN apt-get update && apt-get install -y --no-install-recommends \
    unzip \
    wget \
    && PROTOC_VERSION=31.1 \
    && wget -q https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip -O /tmp/protoc.zip \
    && unzip -q /tmp/protoc.zip -d /usr/local \
    && rm /tmp/protoc.zip \
    && apt-get purge -y --auto-remove unzip wget \
    && rm -rf /var/lib/apt/lists/*

# ============================================
# 第2层: 下载 Go 依赖（go.mod 不变时可缓存）
# ============================================
# 只复制 go.mod 和 go.sum，代码改变时这层不会失效
COPY go.mod go.sum ./
RUN go mod download

# ============================================
# 第3层: 安装 Go 工具（很少变化，高缓存命中率）
# ============================================
# 使用具体版本以避免 goproxy.cn 网络问题
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest \
    && go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest \
    && go install github.com/google/gnostic/cmd/protoc-gen-openapi@v0.7.0 \
    && go install github.com/google/wire/cmd/wire@latest \
    && go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# ============================================
# 第4层: 复制项目代码（经常变化）
# ============================================
# 现在复制所有代码，只有这一步之后的层会在代码修改时失效
COPY . .

# ============================================
# 第5层: 生成 Proto 和 Wire 代码
# ============================================
RUN make proto && make wire

# ============================================
# 第6层: 编译应用
# ============================================
RUN make build

# ============================================
# 运行时镜像（Debian Slim）
# ============================================
FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates  \
        netbase \
        netcat-openbsd \
        default-mysql-client \
        && rm -rf /var/lib/apt/lists/ \
        && apt-get autoremove -y && apt-get autoclean -y

COPY --from=builder /src/bin /app
COPY --from=builder /src/scripts /app/scripts
COPY --from=builder /src/migrations /app/migrations
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate

WORKDIR /app

EXPOSE 8000
EXPOSE 9000
VOLUME /data/conf

CMD ["./QuotaLane", "-conf", "/data/conf"]
