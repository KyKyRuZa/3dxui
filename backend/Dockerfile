FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd

FROM alpine:3.19
RUN apk --no-cache add ca-certificates=20250911-r0 curl
WORKDIR /app
COPY --from=builder /app/server /app/server
COPY --from=builder /app/internal /app/internal

RUN adduser -D -u 1000 app
USER 1000:1000
EXPOSE 8080
CMD ["/app/server"]
