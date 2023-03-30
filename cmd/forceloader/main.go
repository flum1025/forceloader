package main

import (
	"github.com/flum1025/forceloader"
	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() { unitchecker.Main(forceloader.Analyzer) }
