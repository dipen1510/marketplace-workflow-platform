FROM golang:1.26.5-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux \
    go build \
    -o /out/status-controller \
    ./cmd/controller


FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder \
    /out/status-controller \
    /status-controller

ENTRYPOINT ["/status-controller"]