FROM golang:1.19.4-alpine3.16 as builder

ENV GO111MODULE=on \
    GOPROXY=goproxy.io \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=arm64

WORKDIR /app
COPY . .
RUN  go build -o ingress-manager main.go

FROM alpine:3.15.3


WORKDIR /app

COPY --from=builder /app/ingress-manager .

CMD ["./ingress-manager"]