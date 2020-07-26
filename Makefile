.PHONY: nes test nestest apu-test

nes:
	go build ./cmd/nes

test:
	go test ./lib/...
	go build ./test/all-test
	./all-test

count:
	wc -l ./cmd/nes/*.go ./lib/*.go `find test -name "*.go"`
