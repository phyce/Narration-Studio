package response

import (
	"fmt"
	"nstudio/app/common/activity"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/status"
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

type recordedError struct {
	err     error
	chainID uint64
	message string
}

func (e *recordedError) Error() string { return e.err.Error() }

func (e *recordedError) Unwrap() error { return e.err }

func record(severity activity.Severity, summary, detail string, chainID ...uint64) activity.Entry {
	currentStatus := status.Get()

	entryChainID := uint64(0)
	if len(chainID) > 0 {
		entryChainID = chainID[0]
	}

	entry := activity.Record(activity.Entry{
		Severity:    severity,
		Summary:     summary,
		Detail:      detail,
		StatusID:    int(currentStatus.Status),
		StatusLabel: status.Label(currentStatus.Status),
		Source:      callerSource(3),
		ChainID:     entryChainID,
	})
	eventManager.GetInstance().EmitEvent("activity.appended", entry)
	return entry
}

func callerSource(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s:%d", shortFileName(file), line)
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
