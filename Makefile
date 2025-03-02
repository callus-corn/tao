.PHONY: build test clean

build:
	mkdir -p build/
	cd cmd/tao; go build
	mv cmd/tao/tao build/

test:
	cd internal/tftp/; go test

clean:
	rm -rf build/
