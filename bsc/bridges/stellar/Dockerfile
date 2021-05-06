FROM golang:1.16 as BUILDER
WORKDIR /go/src/github.com/threefoldfoundation/tft/bsc/bridges/stellar
COPY ./ ./
RUN go build -ldflags '-w -s -extldflags "-static"'

FROM alpine:3.13.5
COPY --from=BUILDER /go/src/github.com/threefoldfoundation/tft/bsc/bridges/stellar/stellar /
COPY genesis/ /genesis/
ENTRYPOINT [ "/stellar" ]
