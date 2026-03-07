.PHONY: test test-cover lint build

test:
	go test ./...

test-cover:
	go test -coverprofile=cover.out ./... && go tool cover -html=cover.out

lint:
	golangci-lint run

build:
	go build -o ./bin/maintenant ./cmd/maintenant