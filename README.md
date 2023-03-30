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
		--forceloader.restrictedFieldSuffix="UseCase" \
		--forceloader.ignoreResolvers="queryResolver,mutationResolver" \
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
