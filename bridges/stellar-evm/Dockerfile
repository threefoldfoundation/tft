FROM golang:1.20 as BUILDER
WORKDIR /go/src/github.com/threefoldfoundation/tft/bridge/stellar
COPY ./ ./
RUN go build -ldflags '-w -s -extldflags "-static"'

FROM alpine:3.13.5
COPY --from=BUILDER /go/src/github.com/threefoldfoundation/tft/bridge/stellar/stellar /
RUN mkdir /storage
VOLUME /storage
ENTRYPOINT [ "/stellar-evm" ]
