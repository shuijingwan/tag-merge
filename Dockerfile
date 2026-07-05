FROM golang:1.26-alpine

ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=1000

# 安装常用开发工具
RUN apk add --no-cache git bash curl

# 创建普通开发用户，避免容器内生成 root 权限文件
RUN addgroup -g ${USER_GID} ${USERNAME} \
    && adduser -D -u ${USER_UID} -G ${USERNAME} -s /bin/sh ${USERNAME}

# 设置 Go 环境目录权限
RUN mkdir -p /go/pkg/mod /go/bin /home/${USERNAME}/.cache/go-build \
    && chown -R ${USERNAME}:${USERNAME} /go /home/${USERNAME}

WORKDIR /code

USER ${USERNAME}

# 保持容器运行（开发模式）
CMD ["tail", "-f", "/dev/null"]