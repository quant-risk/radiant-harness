# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /radiant ./cmd/radiant/

# Runtime stage
FROM alpine:3.19
RUN apk --no-cache add ca-certificates git
COPY --from=builder /radiant /usr/local/bin/radiant
ENTRYPOINT ["radiant"]
