FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o kiosk-server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /build/kiosk-server .
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/static ./static

RUN addgroup -S kiosk && adduser -S kiosk -G kiosk
RUN chown -R kiosk:kiosk /app
USER kiosk

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 \
  CMD wget -qO- http://localhost:8080/login || exit 1

CMD ["./kiosk-server"]
