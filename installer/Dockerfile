# Multi-stage build for Go installer serive
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download
COPY . ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o installer .

FROM scratch AS runner

WORKDIR /app

COPY --from=builder /app/installer .

EXPOSE 8000
ENTRYPOINT ["./installer"]
