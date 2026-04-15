//go:build cli || clib

package issue

import "fmt"

func Panic(message string, err error) {
	fmt.Printf("[FATAL] %s: %v\n", message, err)
	panic(err)
}
