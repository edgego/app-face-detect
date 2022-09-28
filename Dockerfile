ARG BASE=golang:1.18-alpine3.16
FROM ${BASE} AS builder
ENV GOPROXY=https://goproxy.cn

ARG ALPINE_PKG_BASE="make git gcc g++ libc-dev libsodium-dev zeromq-dev opencv-dev"
ARG ALPINE_PKG_EXTRA=""
ARG ADD_BUILD_TAGS=""

LABEL license='SPDX-License-Identifier: Apache-2.0' \
    copyright='Copyright (c) 2022: EdgeGo'
RUN sed -e 's/dl-cdn[.]alpinelinux.org/mirrors.aliyun.com/g' -i~ /etc/apk/repositories
RUN apk add --update --no-cache ${ALPINE_PKG_BASE} ${ALPINE_PKG_EXTRA}
WORKDIR /app
COPY go.mod vendor* ./
RUN [ ! -d "vendor" ] && go mod download all || echo "skipping..."

ARG MAKE="make -e ADD_BUILD_TAGS=$ADD_BUILD_TAGS build"
COPY . .
RUN $MAKE

#final stage
FROM alpine:3.16
LABEL license='SPDX-License-Identifier: Apache-2.0' \
  copyright='Copyright (c) 2022: EdgeGo'
LABEL Name=app-face-detect Version=${VERSION}

# dumb-init is required as security-bootstrapper uses it in the entrypoint script
RUN sed -e 's/dl-cdn[.]alpinelinux.org/mirrors.aliyun.com/g' -i~ /etc/apk/repositories
RUN apk add --update --no-cache ca-certificates zeromq dumb-init opencv

#COPY --from=builder /app/Attribution.txt /Attribution.txt
#COPY --from=builder /app/LICENSE /LICENSE
COPY --from=builder /app/res/ /res/
COPY --from=builder /app/model/ /model/
COPY --from=builder /app/app-face-detect /app-face-detect

EXPOSE 48098

# Must always specify the profile using
# environment:
#   - EDGEX_PROFILE: <profile>
# or use
# command: "-profile=<profile>"
# If not you will recive error:
# SDK initialization failed: Could not load configuration file (./res/configuration.toml)...

ENTRYPOINT ["/app-face-detect"]
CMD ["-cp=consul.http://edgex-core-consul:8500", "--registry", "--confdir=/res"]