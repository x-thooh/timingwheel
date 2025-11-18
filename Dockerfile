#FROM registry.cn-chengdu.aliyuncs.com/lsxd/golang:1.21.13-bullseye-build AS builder
FROM golang:1.24.7 AS builder

COPY . /src
WORKDIR /src

RUN GOPROXY=https://goproxy.cn,direct make build

FROM alpine

ARG SERVICE_NAME=delay
ENV SERVICE_NAME=${SERVICE_NAME}

RUN apk add --no-cache tzdata \
    && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

COPY --from=builder /src/bin/${SERVICE_NAME} /app
COPY --from=builder /src/configs /app/configs


EXPOSE 8081
VOLUME /data/conf

CMD ["/app/${SERVICE_NAME}", "-conf", "./configs"]
