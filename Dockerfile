# Estágio 1: Builder (A Cozinha)
FROM golang:1.23-alpine AS builder

ARG SERVICE_NAME

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY go.mod ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/executavel_final ./${SERVICE_NAME}/main.go

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /app/executavel_final /executavel_final

ENTRYPOINT ["/executavel_final"]