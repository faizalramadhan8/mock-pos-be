FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bakeshop-be ./cmd/backend

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /bakeshop-be .
COPY migrations ./migrations

EXPOSE 7889
CMD ["./bakeshop-be"]
