# ---- Frontend ----
FROM node:20-alpine AS frontend

RUN apk add --no-cache git

RUN git clone https://github.com/mikecurious/design-spark /frontend

WORKDIR /frontend

ARG VITE_API_BASE_URL=https://shop.dominicatechnologies.com
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL

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
