# ==========================================
# Stage 1: Build the Go Binary
# ==========================================

FROM golang:alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o uno-server cmd/server/main.go

# ==========================================
# Stage 2: Minimal Production Image
# ==========================================


FROM alpine:3.21

RUN apk --no-cache add ca-certificates && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/uno-server .
COPY --from=builder /app/web ./web

USER appuser

EXPOSE 8080

CMD ["./uno-server"]