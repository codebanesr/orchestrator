.PHONY: all build run clean swagger

all: swagger build

swagger:
	swag init --parseDependency --parseInternal

build: swagger
	go build -o bin/orchestrator

run: swagger
	go run main.go

clean:
	rm -rf bin/
	rm -rf docs/