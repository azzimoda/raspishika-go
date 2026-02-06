FROM golang:1.25-bookworm

RUN apt-get update && apt-get install -y nodejs

WORKDIR /app

RUN go run github.com/playwright-community/playwright-go/cmd/playwright@v0.5200.0 install chromium chromium-headless-shell --with-deps
RUN rm -rf /var/lib/apt/lists/*

COPY . .
RUN go build -o raspishika ./cmd/cli/main.go

ENTRYPOINT ["/app/raspishika"]
