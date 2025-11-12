FROM golang:1.24 AS builder

COPY . /src
WORKDIR /src

# Install Wire for dependency injection code generation
RUN go install github.com/google/wire/cmd/wire@latest

# Generate Wire code before building
RUN make wire

# Build the application
RUN GOPROXY=https://goproxy.cn make build

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
		ca-certificates  \
        netbase \
        && rm -rf /var/lib/apt/lists/ \
        && apt-get autoremove -y && apt-get autoclean -y

COPY --from=builder /src/bin /app

WORKDIR /app

EXPOSE 8000
EXPOSE 9000
VOLUME /data/conf

CMD ["./QuotaLane", "-conf", "/data/conf"]
