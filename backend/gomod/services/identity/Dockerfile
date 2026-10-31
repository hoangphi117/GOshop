# ============= STAGE 1: BUILD GO BINARY =============
FROM golang:1.26.6-alpine AS builder

RUN apk update && apk add --no-cache git curl ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o identity ./cmd/identity

# ============= STAGE 2: RUNTIME CONTAINER =============
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/identity .

EXPOSE 8081 50051

CMD ["./identity"]
