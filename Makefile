.PHONY: build run clean test

build:
	go build -o monchecker cmd/monitor/main.go

run: build
	./monchecker

clean:
	rm -f monchecker
	rm -rf data/metrics/*
	rm -rf data/anomalies/*

test:
	go test -v ./...

deps:
	go mod tidy
	go mod download 