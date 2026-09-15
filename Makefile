BINARY := json-log-parser
BIN_DIR := bin

.PHONY: build install clean

build:
	go build -o $(BIN_DIR)/$(BINARY) .

install:
	go install .

clean:
	rm -rf $(BIN_DIR)
