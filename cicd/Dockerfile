#syntax=docker/dockerfile-upstream:1.5
ARG GO_APP

FROM alpine:3.18.2 as deps

ARG GO_APP

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

RUN mkdir -p /go/bin /go/src

COPY --from=build /go/src/dist /go/src/dist

RUN <<EOT
  set -e 
  apk add --no-cache ca-certificates jq
  if [[ ${TARGETARCH} == "arm" ]]; then VARIANT=$(echo ${TARGETVARIANT} | sed 's/^v//'); fi
  BIN_PATH=$(jq -r ".[] |select(.type   == \"Binary\" and \
                                .name   == \"${GO_APP}\" and \
                                .goos   == \"${TARGETOS}\" and \
                                .goarch == \"${TARGETARCH}\" and \
                                (.goarm == \"${VARIANT}\" or .goarm == null)) | .path" < /go/src/dist/artifacts.json)
  cp ${BIN_PATH} /go/bin
EOT

FROM alpine:3.18.2

ARG GO_APP
ENV GO_APP ${GO_APP}

COPY --from=deps --chmod=755 /go/bin/${GO_APP} /usr/local/bin/${GO_APP}

COPY --from=deps /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=assets entrypoint.sh /entrypoint.sh

RUN ln -s /usr/local/bin/${GO_APP} /${GO_APP} && chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
