# ---- Frontend ----
FROM node:20-alpine AS frontend

RUN apk add --no-cache git

RUN git clone https://github.com/mikecurious/design-spark /frontend

WORKDIR /frontend

ARG VITE_API_BASE_URL=https://shop.dominicatechnologies.com
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL

# Replace Lovable branding with Dominica Shop branding
COPY favicon.svg /frontend/public/favicon.svg
RUN rm -f /frontend/public/favicon.ico && \
    sed -i \
      -e 's|<title>Lovable App</title>|<title>Dominica Shop</title>|' \
      -e 's|content="Lovable Generated Project"|content="Dominica Shop — Kiosk Management System"|' \
      -e 's|content="Lovable App"|content="Dominica Shop"|' \
      -e 's|content="Lovable"|content="Dominica Technologies"|' \
      -e 's|content="@Lovable"|content="@DominicaTech"|' \
      -e 's|content="https://lovable.dev/opengraph-image-p98pqg.png"||g' \
      -e 's|<meta name="viewport"|<link rel="icon" href="/favicon.svg" type="image/svg+xml" />\n    <meta name="viewport"|' \
      /frontend/index.html

RUN npm install
RUN npm run build

# ---- Backend ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o kiosk-server ./cmd/server

# ---- Final ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /build/kiosk-server .
COPY --from=frontend /frontend/dist ./dist

RUN addgroup -S kiosk && adduser -S kiosk -G kiosk
RUN chown -R kiosk:kiosk /app
USER kiosk

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
  CMD wget -qO- http://localhost:8080/ || exit 1

CMD ["./kiosk-server"]
