.PHONY: build run test docker-build clean

build:
	go build -o bin/geoserv ./cmd/geoserv

run: build
	./bin/geoserv

test:
	go test ./...

docker-build:
	docker build -t geoserv:local .

clean:
	rm -rf bin/
