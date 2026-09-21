APP=m-ui

.PHONY: build test run
build:
	CGO_ENABLED=0 go build -trimpath -o $(APP) .

test:
	go test ./...

run:
	go run .
