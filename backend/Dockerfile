# Multi-stage build for Go backend
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . ./

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server ./cmd/server

FROM scratch AS runner

WORKDIR /app

COPY --from=builder /app/server .

EXPOSE 8080
ENTRYPOINT ["./server"]
