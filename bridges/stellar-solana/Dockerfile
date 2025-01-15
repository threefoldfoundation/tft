FROM golang:1.21 as BUILDER
WORKDIR /go/src/github.com/threefoldfoundation/tft/bridges/stellar-solana
COPY ./ ./
RUN go build -ldflags '-w -s -extldflags "-static"'

FROM alpine:3.13.5
COPY --from=BUILDER /go/src/github.com/threefoldfoundation/tft/bridges/stellar-solana/stellar-solana /
RUN mkdir /storage
VOLUME /storage
ENTRYPOINT [ "/stellar-solana" ]
