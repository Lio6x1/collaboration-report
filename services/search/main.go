package main

import (
	"collaboration/platform"
	"collaboration/search/internal/bootstrap"
)

func main() {
	platform.Main("search", bootstrap.RegisterSearch, nil)
}
