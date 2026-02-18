.PHONY: build test clean

build:
	CGO_ENABLED=0 go build -o agent-spy .

test:
	CGO_ENABLED=0 go test ./... -v -timeout 60s

clean:
	rm -f agent-spy

install:
	CGO_ENABLED=0 go install .
