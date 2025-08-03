.PHONY: nes nsf test nestest apu-test make-screenshot mapper static

nes:
	time go build ./cmd/nes

nsf:
	go build ./cmd/nsf

mapper:
	go build ./cmd/mapper

test:
	go test ./lib/... ./cmd/...
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

static:
	go build -tags static -ldflags "-s -w" -o nes ./cmd/nes

docker:
	docker build -f Dockerfile -t nes-static:latest .
	docker run --rm -v "$(PWD):/mnt" nes-static
