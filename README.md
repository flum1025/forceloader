# forceloader

This linter enforces calling dataloaders within resolvers. Prohibits any other data operations outside of dataloaders.

## Install

```
$ go install github.com/flum1025/forceloader/cmd/forceloader@latest
```

## Run

```sh
$ go vet \
		-vettool=$(which forceloader) \
		--forceloader.resolverStruct="a.Resolver" \
		--forceloader.restrictedPackages="a/usecase" \
		--forceloader.ignoreResolverStructs="a.queryResolver,a.mutationResolver" \
		./...
```

or

```sh
$ go build -buildmode=plugin -o plugin.so ./plugin
```

In .golangci.yml

```yml
linters:
  enable:
    - forceloader

linters-settings:
  custom:
    forceloader:
      path: ./plugin.so
```
