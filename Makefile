.PHONY: nes test nestest

nes:
	go build ./cmd/nes

nestest:
	go build ./test/nestest

test:
	go test ./...

count:
	wc -l ./cmd/nes/*.go
