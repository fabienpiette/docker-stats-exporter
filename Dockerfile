# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build
RUN apk add --no-cache git make

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o docker-stats-exporter \
    ./cmd/exporter

# Final stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -g 1000 exporter && \
    adduser -D -u 1000 -G exporter exporter

WORKDIR /app
COPY --from=builder /build/docker-stats-exporter .
RUN chown -R exporter:exporter /app

USER exporter
EXPOSE 9200

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9200/health || exit 1

ENTRYPOINT ["./docker-stats-exporter"]
