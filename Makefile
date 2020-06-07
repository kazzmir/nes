.PHONY: nes

nes:
	go build ./cmd/nes

test:
	go test ./...

count:
	wc -l ./cmd/nes/*.go
