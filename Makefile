.PHONY: nes

nes:
	go build ./cmd/nes

test:
	go test ./...
