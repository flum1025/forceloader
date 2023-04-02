.PHONY: dummy build test

dummy:
	@echo argument is required

build:
	go build cmd/forceloader/main.go

run: build
	cd ./testdata/src/a && \
		go vet \
		-vettool=../../../main \
		--forceloader.resolverStruct="a.Resolver" \
		--forceloader.restrictedPackages="a/usecase" \
		--forceloader.ignoreResolverStructs="a.queryResolver,a.mutationResolver" \
		./...

test:
	go test ./...
