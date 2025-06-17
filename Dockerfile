FROM golang:1.21

WORKDIR /app

COPY . .

RUN go mod tidy
RUN go build -o auth-service

EXPOSE 50051

CMD ["./auth-service"]
