.PHONY: build test lint build-all clean

build:
	CGO_ENABLED=0 go build -v -o bin/statix ./cmd/statix

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

build-all:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o bin/statix-linux-amd64 ./cmd/statix
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -v -o bin/statix-linux-arm64 ./cmd/statix

clean:
	rm -rf bin/
