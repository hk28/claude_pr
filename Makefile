.PHONY: build run

build:
	templ generate
	go build -o prman .

run: build
	./prman -config ./config-local
