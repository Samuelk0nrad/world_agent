FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/agent-backend ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/agent-backend /app/agent-backend

RUN addgroup -S app && adduser -S app -G app && mkdir -p /app/data && chown -R app:app /app

USER app

ENV AGENT_BACKEND_PORT=8088
ENV AGENT_MEMORY_FILE=/app/data/memory.jsonl

EXPOSE 8088

CMD ["/app/agent-backend"]
