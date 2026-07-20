.PHONY: build install test vet clean

BINARY := bin/narrowmap

build:
	mkdir -p bin
	go build -trimpath -o $(BINARY) ./cmd/narrowmap

install:
	go install ./cmd/narrowmap

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
