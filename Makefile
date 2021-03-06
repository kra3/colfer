include common.mk

.PHONY: dist
dist: clean test build
	go fmt ./...
	go vet ./...

.PHONY: test
test: install
	make -C c
	make -C ecma
	make -C go
	make -C java
	make -C rpc
	# Fails on Travis CI: mvn -f java/maven integration-test

.PHONY: bench
bench: install
	make -C c/bench
	make -C go/bench
	make -C java/bench

build:
	GOARCH=amd64 GOOS=linux go build -o build/colf-linux ./cmd/colf
	GOARCH=amd64 GOOS=darwin go build -o build/colf-darwin ./cmd/colf
	GOARCH=amd64 GOOS=openbsd go build -o build/colf-openbsd ./cmd/colf
	GOARCH=amd64 GOOS=windows go build -o build/colf.exe ./cmd/colf

.PHONY: clean
clean:
	go clean -i ./cmd/...
	rm -fr build
	# Fails on Travis CI:  mvn -f java/maven clean
