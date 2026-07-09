# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/agentpulse-server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/agentpulse-server /app/agentpulse-server
COPY backend/configs/config.example.yaml /app/configs/config.example.yaml
USER nonroot:nonroot
EXPOSE 8080 4318
ENTRYPOINT ["/app/agentpulse-server"]
CMD ["-config", "/app/configs/config.yaml"]
