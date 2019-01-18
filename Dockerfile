FROM golang:1.10-alpine3.8 as builder
LABEL maintainer="v.zorin@anchorfree.com"

RUN apk add --no-cache git
COPY cmd /go/src/github.com/anchorfree/ssl-watch/cmd
COPY Gopkg.toml /go/src/github.com/anchorfree/ssl-watch/

RUN cd /go && go get -u github.com/golang/dep/cmd/dep
RUN cd /go/src/github.com/anchorfree/ssl-watch/ && dep ensure
RUN cd /go && go build github.com/anchorfree/ssl-watch/cmd/ssl-watch

FROM alpine:3.8
LABEL maintainer="v.zorin@anchorfree.com"

RUN apk add --no-cache ca-certificates
COPY --from=builder /go/ssl-watch /usr/local/bin/ssl-watch

ENTRYPOINT ["/usr/local/bin/ssl-watch"]
