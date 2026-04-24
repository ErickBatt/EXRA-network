BINARY  := exra-server
MAIN    := .

.PHONY: build build-linux build-arm64 clean

build:
	go build -o $(BINARY) $(MAIN)

# Cross-compile for Linux x86_64 (standard cloud VMs)
build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)-linux-amd64 $(MAIN)

# Cross-compile for Linux ARM64 (Graviton, Ampere, Pi)
build-arm64:
	GOOS=linux GOARCH=arm64 go build -o $(BINARY)-linux-arm64 $(MAIN)

clean:
	rm -f $(BINARY) $(BINARY)-linux-amd64 $(BINARY)-linux-arm64
