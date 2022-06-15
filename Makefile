.PHONY: nes nsf test nestest apu-test make-screenshot

nes:
	time go build ./cmd/nes

nsf:
	go build ./cmd/nsf

test:
	go test ./lib/...
	go build ./test/all-test
	./all-test

make-screenshot:
	go build ./test/make-screenshot

count:
	wc -l `find cmd -name "*.go"` `find lib -name "*.go"` `find test -name "*.go"` `find util -name "*.go"`

all: nes nsf make-screenshot test

update:
	go get -u ./...
	go mod tidy
