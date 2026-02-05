FROM golang:1.21-alpine

RUN apk add --no-cache git curl

WORKDIR /app
COPY . .

RUN go build -o depextify cmd/depextify/main.go && \
    ls -l depextify

RUN ["/bin/sh", "-c", "echo 'Hello from exec form'"]

CMD ["./depextify"]
