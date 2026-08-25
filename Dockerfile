FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/mflint-server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/mflint-server ./mflint-server
COPY web ./web

ENV WEB_DIR=/app/web
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["./mflint-server"]
