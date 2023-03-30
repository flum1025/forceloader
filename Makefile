.PHONY: dummy build test

dummy:
	@echo argument is required

build:
	go build cmd/forceloader/main.go

run: build
	cd ./testdata/src/a && \
		go vet \
		-vettool=../../../main \
		--forceloader.restrictedFieldSuffix="UseCase" \
		--forceloader.ignoreResolvers="queryResolver,mutationResolver" \
		./...

test:
	go test ./...
