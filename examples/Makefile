all: test build

test:
	go test ./...
	echo "Testing done"

build:
	@echo "Building..."
	go build -o depextify cmd/depextify/main.go
	ls -l depextify

deploy:
	curl -X POST https://example.com/deploy
