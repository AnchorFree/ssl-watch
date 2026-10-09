# syntax=docker/dockerfile:1

# Build stage. Pinned by digest.
FROM golang:1.26.9@sha256:f1f0bcc2c524a3ced375fcb4d1ecb7aa371aa7070e112599aaca45cc02d0101b AS builder

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

# Final container stage. Pinned by digest, so a rebuild cannot silently pick up a different base.
FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

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
