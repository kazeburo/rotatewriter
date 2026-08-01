all: check

check:
	go test -v ./...

lint:
	golangci-lint run ./...