package issue

import (
	"fmt"
	"runtime"
	"strings"
)

func Trace(err error) error {
	if err == nil {
		return nil
	}

	_, file, line, ok := runtime.Caller(1)
	if !ok {
		panic(fmt.Errorf("Trace failed: %v", err))
	}

	shortFile := shortFileName(file)
	traceLine := fmt.Sprintf("%s:%d", shortFile, line)

	result := fmt.Errorf("%v\n%s", err, traceLine)

	return result
}

func Panic(err error) {
	/*
		Add some form of crash dump/log
	*/
	panic(Trace(err))
}

func shortFileName(fullPath string) string {
	lastSlash := strings.LastIndex(fullPath, "/")
	if lastSlash == -1 {
		lastSlash = strings.LastIndex(fullPath, "\\")
	}
	if lastSlash == -1 {
		return fullPath
	}
	return fullPath[lastSlash+1:]
}
