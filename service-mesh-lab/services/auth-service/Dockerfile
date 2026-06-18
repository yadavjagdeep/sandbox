FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o auth-service .

FROM alpine:3.20
WORKDIR /app
COPY --from=builder app/auth-service .
EXPOSE 9000
CMD [ "./auth-service" ]