build:
	go build -o bin/

run: build
	./bin/anonymous-chat