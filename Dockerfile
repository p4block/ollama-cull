# Stage 1: Build
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o culler cull.go

# Stage 2: Distroless
FROM gcr.io/distroless/base-debian12:nonroot

WORKDIR /app
COPY --from=builder /app/culler .
COPY --from=builder /app/.env.example .

ENTRYPOINT ["/app/culler"]