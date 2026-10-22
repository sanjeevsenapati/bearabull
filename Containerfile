# Containerfile for Bearabull Platform (Podman)
# Stage 1: Build binary
FROM docker.io/library/golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bearabull ./cmd/bearabull

# Stage 2: Minimal runtime image
FROM docker.io/library/alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Kolkata

COPY --from=builder /app/bearabull /app/bearabull
COPY --from=builder /app/config.json /app/config.json

RUN mkdir -p /app/data
ENV DATABASE_PATH=/app/data/bearabull.db
ENV APP_PORT=8080

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/v1/health || exit 1

ENTRYPOINT ["/app/bearabull"]
CMD ["-config", "/app/config.json"]
