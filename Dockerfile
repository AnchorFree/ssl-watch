FROM golang:1.14-alpine3.11 as builder
LABEL maintainer="v.zorin@anchorfree.com"

RUN apk add --no-cache git
COPY cmd /cmd
RUN cd /cmd && go build

FROM alpine:3.11
LABEL maintainer="v.zorin@anchorfree.com"

RUN apk add --no-cache ca-certificates
COPY --from=builder /cmd/ssl-watch /usr/local/bin/ssl-watch

ENTRYPOINT ["/usr/local/bin/ssl-watch"]
