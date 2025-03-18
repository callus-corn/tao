.PHONY: build test clean

build:
	mkdir -p build/
	cd cmd/tao; go build
	mv cmd/tao/tao build/

test:
	cd internal/tftp/; go test

bench:
	cd internal/tftp/; go test -run Benchmark* -bench . -benchmem

clean:
	rm -rf build/
