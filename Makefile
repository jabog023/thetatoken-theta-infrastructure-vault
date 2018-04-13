GOTOOLS =	github.com/mitchellh/gox \
			github.com/Masterminds/glide \
			github.com/rigelrozanski/shelldown/cmd/shelldown
			
all: get_vendor_deps install test

build:
	go build

install:
	go install

test: test_unit

test_unit:
	go test `glide novendor`

get_vendor_deps: tools
	glide install

tools:
	@go get $(GOTOOLS)

gen_mocks:
	# go get github.com/vektra/mockery/.../
	mockery -name KeyManager -case=underscore -inpkg
	mockery -name RPCClient -case=underscore -inpkg

clean:
	# maybe cleaning up cache and vendor is overkill, but sometimes
	# you don't get the most recent versions with lots of branches, changes, rebases...
	@rm -rf ./vendor
	@rm -f $GOPATH/bin/vault

.PHONY: all build install test test_unit get_vendor_deps clean tools gen_mocks
