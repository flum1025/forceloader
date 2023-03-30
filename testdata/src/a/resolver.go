package a

import (
	"a/loader"
	"a/usecase"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	Loader   loader.Loader
	UseCase  usecase.UseCase
	UseCase2 usecase.UseCase
}
