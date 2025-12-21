package response

import (
	"fmt"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/util"
	"nstudio/app/config"
	"runtime"
	"strings"
)

func emitEvent(name string, data util.MessageData, log bool) {
	if config.Debug() {
		fmt.Println(fmt.Sprintf("event: %s - %s - %s ", data.Severity, data.Summary, data.Detail))
	}
	if notificationEnabled {
		eventManager.GetInstance().EmitEvent(name, data)
	}
}

func trace(err error) error {
	if err == nil {
		return nil
	}

	_, file, line, ok := runtime.Caller(2)
	if !ok {
		panic(fmt.Errorf("Trace failed: %v", err))
	}

	shortFile := shortFileName(file)
	traceLine := fmt.Sprintf("%s:%d", shortFile, line)

	result := fmt.Errorf("%v\n%s", err, traceLine)

	return result
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
