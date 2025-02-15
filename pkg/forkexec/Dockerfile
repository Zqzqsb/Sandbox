FROM golang:latest

WORKDIR /go/src/reproduce_sanbox
COPY ../../../. .

# 安装必要的工具
RUN apt-get update && apt-get install -y \
    procps \
    && rm -rf /var/lib/apt/lists/*

# 设置环境变量
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GOPROXY=https://goproxy.cn,direct

# 获取依赖
RUN go mod download

# 默认运行测试
WORKDIR /go/src/reproduce_sanbox/pkg/forkexec
CMD ["go", "test", "-bench=.", "-benchmem", "."]
