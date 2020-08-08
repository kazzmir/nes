.PHONY: nes test nestest apu-test make-screenshot

nes:
	go build ./cmd/nes

test:
	go test ./lib/...
	go build ./test/all-test
	./all-test

make-screenshot:
	go build ./test/make-screenshot

count:
	wc -l ./cmd/nes/*.go ./lib/*.go `find test -name "*.go"`
