.PHONY: build install test clean

build:
	mkdir -p build/
	cd cmd/tao; go build
	mv cmd/tao/tao build/

install: build/tao tao.conf systemd/tao.service
	mkdir -p /var/lib/tao/
	mkdir -p /etc/tao/
	chmod 755 /var/lib/tao/
	chmod 755 /etc/tao/
	chown root:root /var/lib/tao/
	chown root:root /etc/tao/
	cp tao.conf /etc/tao/tao.conf
	cp build/tao /usr/bin/tao
	cp systemd/tao.service /usr/lib/systemd/system/tao.service
	chmod 755 /etc/tao/tao.conf
	chmod 755 /usr/bin/tao
	chmod 755 /usr/lib/systemd/system/tao.service
	chown root:root /etc/tao/tao.conf
	chown root:root /usr/bin/tao
	chown root:root /usr/lib/systemd/system/tao.service

test:
	cd internal/tftp/; go test

bench:
	cd internal/tftp/; go test -run Benchmark* -bench . -benchmem

clean:
	rm -rf build/
