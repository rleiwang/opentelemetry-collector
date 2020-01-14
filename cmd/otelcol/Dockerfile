FROM golang:1.13.5-alpine3.11 AS builder
LABEL autopurge=true

# Install git + SSL ca certificates.
# Git is required for fetching the dependencies.
# Ca-certificates is required to call HTTPS endpoints.
# https://bugs.alpinelinux.org/issues/9935
RUN apk update && apk add --no-cache git ca-certificates tzdata yarn alpine-sdk cmake && update-ca-certificates

COPY . opentelemetry

WORKDIR ./opentelemetry
RUN go mod tidy
#https://github.com/valyala/gozstd/issues/20#issuecomment-557499034
RUN BUILD_DIR=$(pwd); GOZSTD_VER=$(cat go.mod | fgrep github.com/valyala/gozstd | awk '{print $NF}'); go get -d github.com/valyala/gozstd@${GOZSTD_VER}; cd ${GOPATH}/pkg/mod/github.com/valyala/gozstd@${GOZSTD_VER}; if [[ ! -f _rebuilt ]]; then chmod -R +w .; make -j8 clean; make -j8 libzstd.a; touch _rebuilt; fi; cd ${BUILD_DIR}
RUN make otelcol

#
FROM scratch AS otelcol

# Import from builder.
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd

# Copy static executable
COPY --from=builder /go/opentelemetry/cmd/otelcol/otelcol /usr/local/bin/otelcol

# Copy libs for zstd
COPY --from=builder /lib/ld-musl-x86_64.so.1 /lib/ld-musl-x86_64.so.1

ENTRYPOINT ["/usr/local/bin/otelcol"]
EXPOSE 55678 55679
