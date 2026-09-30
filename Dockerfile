# syntax=docker/dockerfile:1

# Build stage. Pinned by digest to the same golang gpr-edge and ula-edge use.
FROM golang:1.26.8@sha256:9d2f36f06329b2a141b9db99ffa32765cf695ee57b813ca29e245e8670bcbfff AS builder

ENV BASE_DIR=/go/src/ssl-watch

# Warming modules cache with project dependencies
WORKDIR ${BASE_DIR}
COPY go.mod go.sum ./
RUN go mod download

# Copy project source code to WORKDIR
COPY . .

# Run tests and build on success. -buildvcs=false keeps the build off the copied .git
# directory, which Go would otherwise stamp from and reject as dubiously owned.
RUN go test ./... \
 && CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -a -tags netgo -ldflags '-w'

# Final container stage. Pinned by digest to the same alpine gpr-edge and ula-edge use, so a
# rebuild cannot silently pick up a different base.
FROM alpine:3.24.1@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b

# ca-certificates for the S3 client. The endpoint checks skip verification on purpose.
RUN apk add --no-cache ca-certificates

# Run as a non-root uid. ssl-watch reads its config from S3 or from a read-only ConfigMap
# mount at /etc/ssl-watch, writes nothing to disk, and binds 9105. The chart sets no pod
# runAsUser, so the projected IRSA token keeps its default 0644 mode and uid 10001 can read it.
RUN addgroup -g 10001 -S ssl-watch \
 && adduser -u 10001 -S -G ssl-watch ssl-watch

COPY --from=builder /go/src/ssl-watch/ssl-watch /usr/local/bin/ssl-watch

EXPOSE 9105
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/ssl-watch"]
