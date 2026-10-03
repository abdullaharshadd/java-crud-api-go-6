FROM golang:1.25-alpine

WORKDIR /app

COPY . .

RUN cd /app && mkdir -p /app/bin && go mod tidy && go mod download && go build -o /app/bin/server ./cmd/server

EXPOSE 8080

CMD ["sh", "-c", "/app/bin/server"]
