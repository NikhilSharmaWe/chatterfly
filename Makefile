build:
	@go build -o bin/chatterfly

run: build
	@./bin/chatterfly