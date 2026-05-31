.PHONY: build test clean run dev

BINARY=paylinkd
BUILD_DIR=./build
DATA_DIR=./data

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/paylinkd

test:
	go test ./... -v -count=1

clean:
	rm -rf $(BUILD_DIR) $(DATA_DIR)

run: build
	$(BUILD_DIR)/$(BINARY) --datadir $(DATA_DIR)

dev: build
	$(BUILD_DIR)/$(BINARY) --datadir $(DATA_DIR) --block-interval 2000

lint:
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy
