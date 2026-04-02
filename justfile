version := `git describe --tags --always --dirty 2>/dev/null || echo dev`

build:
    go build -ldflags "-s -w -X main.version={{version}}" -o bin/meta ./cmd/meta

install:
    go install -ldflags "-s -w -X main.version={{version}}" ./cmd/meta

lint:
    golangci-lint run

test:
    go test ./...

clean:
    rm -rf bin/
