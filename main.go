package main

import (
	v8 "github.com/renniemaharaj/go-api-kit/v8"
)

func main() {
	if v8 := v8.Run("dist", "localhost", "9100"); v8 != nil {
		v8()
	}
}
