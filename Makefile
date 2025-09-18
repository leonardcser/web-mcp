SHELL := /bin/sh

.PHONY: all server cache clean

all: server cache

server:
	go build -o web-mcp ./cmd/server

cache:
	go build -o web-mcp-cache ./cmd/cache-server

clean:
	rm -f web-mcp web-mcp-cache


