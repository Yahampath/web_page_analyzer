# STAGE 1
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /web_page_analyzer

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o web_page_analyzer main.go

# STAGE 2
FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /web_page_analyzer

COPY --from=builder /web_page_analyzer/web_page_analyzer .
COPY --from=builder /web_page_analyzer/config.env .

EXPOSE 8090 9090 6060

ENTRYPOINT ["./web_page_analyzer"]
