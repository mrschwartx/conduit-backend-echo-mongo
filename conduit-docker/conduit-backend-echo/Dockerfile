FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/backend ./cmd/migrate ./cmd/seed

FROM alpine:3.22 AS runner
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
ENV APP_HOST=0.0.0.0 \
    APP_PORT=8080 \
    LOG_TO_FILE=false
COPY --from=build /out/backend /out/migrate /out/seed /app/
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/health || exit 1
CMD ["/app/backend"]
