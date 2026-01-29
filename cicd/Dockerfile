#syntax=docker/dockerfile:1.21
ARG GO_APP

FROM alpine:3.23.3 AS deps

ARG GO_APP
ARG GORELEASER_DIST_DIR=/go/src/dist

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

RUN <<EOT
  set -e
  apk add --no-cache ca-certificates jq
  mkdir -p /go/bin /go/src ${GORELEASER_DIST_DIR}
EOT

COPY --from=build ${GORELEASER_DIST_DIR}/ ${GORELEASER_DIST_DIR}

WORKDIR /go/src

RUN <<EOT
  set -e

  if [ "${TARGETARCH}" = "arm" ]; then VARIANT=$(echo "${TARGETVARIANT}" | sed 's/^v//'); fi
  BIN_PATH=$(jq -r ".[] |select(.type   == \"Binary\" and \
                                .name   == \"${GO_APP}\" and \
                                .goos   == \"${TARGETOS}\" and \
                                .goarch == \"${TARGETARCH}\" and \
                                (.goarm == \"${VARIANT}\" or .goarm == null)) | .path" < ${GORELEASER_DIST_DIR}/artifacts.json)

  if [ -z "${BIN_PATH}" ]; then
    echo "ERROR: Binary not found for ${GO_APP} ${TARGETOS}/${TARGETARCH}/${TARGETVARIANT}"
    exit 1
  fi

  cp ${BIN_PATH} /go/bin
EOT

FROM alpine:3.23.3

ARG GO_APP
ENV GO_APP=${GO_APP}

COPY --from=deps --chmod=755 /go/bin/${GO_APP} /usr/local/bin/${GO_APP}

COPY --from=deps /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=assets --chmod=755 entrypoint.sh /entrypoint.sh

RUN ln -s /usr/local/bin/${GO_APP} /${GO_APP}

ENTRYPOINT ["/entrypoint.sh"]
