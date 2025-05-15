# build stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o funding-screener main.go

# minimal runtime image
FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache tzdata
COPY --from=builder /app/funding-screener .
CMD ["./funding-screener"] 