FROM golang:1.16
WORKDIR /go/src/github.com/threefoldfoundation/tft/bsc/bridges/stellar
COPY ./ ./
RUN go build -ldflags '-w -s -extldflags "-static"'

FROM alpine:latest
COPY --from=0 /go/src/github.com/threefoldfoundation/tft/bsc/bridges/stellar/stellar /
COPY genesis/ /genesis/
ENTRYPOINT [ "/stellar" ]
