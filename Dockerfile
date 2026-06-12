FROM golang:1.26-alpine

# 安装 git（及其他常用工具，可选）
RUN apk add --no-cache git

# 设置工作目录（与 docker-compose 中的 working_dir 一致）
WORKDIR /code

# 保持容器运行（开发模式）
CMD ["tail", "-f", "/dev/null"]
