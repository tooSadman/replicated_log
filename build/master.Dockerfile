FROM golang:1.17.2-alpine3.14 AS gobuild
RUN apk add --no-cache build-base
COPY go.* /tmp/go-build/
COPY cmd /tmp/go-build/cmd/
COPY internal /tmp/go-build/internal/
RUN cd /tmp/go-build \
    && go build -a -tags musl cmd/master/main.go

FROM alpine:3.14.2 AS base
COPY --from=gobuild /tmp/go-build/main /usr/local/bin/rl-master
CMD ["/usr/local/bin/rl-master"]
