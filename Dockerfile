# build stage
FROM golang:1.24.2-alpine AS builder
WORKDIR /app
RUN apk add --no-cache protobuf
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.33.0
COPY . .
RUN protoc --go_out=./packages/transport-bus/proto --go_opt=paths=source_relative ./packages/transport-bus/proto/funding.proto
RUN go build -o funding-screener main.go

# minimal runtime image
FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache tzdata
COPY --from=builder /app/funding-screener .
CMD ["./funding-screener"] 