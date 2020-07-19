.PHONY: nes test nestest apu-test

nes:
	go build ./cmd/nes

nestest:
	go build ./test/nestest

apu-test:
	go build ./test/apu-test

test:
	go test ./lib/...

count:
	wc -l ./cmd/nes/*.go ./lib/*.go
