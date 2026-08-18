FROM golang:1.26.5-alpine AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 \
    GOOS=linux \
    go build \
    -o /out/marketplace-worker \
    ./cmd/worker


FROM alpine:3.22

RUN addgroup -S marketplace && \
    adduser -S marketplace -G marketplace

COPY --from=builder \
    /out/marketplace-worker \
    /usr/local/bin/marketplace-worker

USER marketplace

ENTRYPOINT ["/usr/local/bin/marketplace-worker"]