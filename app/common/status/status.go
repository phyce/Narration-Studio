package status

import (
	"nstudio/app/common/activity"
	"nstudio/app/common/eventManager"
)

type ID int

const (
	Unknown ID = iota
	Loading
	Ready
	Streaming
	Generating
	Playing
	Error
	Warning
)

type StatusEvent struct {
	Status  ID     `json:"status"`
	Message string `json:"message"`
}

var currentStatus = StatusEvent{
	Status:  Unknown,
	Message: "",
}

func Label(status ID) string {
	switch status {
	case Loading:
		return "Loading"
	case Ready:
		return "Ready"
	case Streaming:
		return "Streaming"
	case Generating:
		return "Generating"
	case Playing:
		return "Playing"
	case Error:
		return "Error"
	case Warning:
		return "Warning"
	default:
		return "Unknown"
	}
}

func Set(status ID, message string) {
	currentStatus = StatusEvent{
		Status:  status,
		Message: message,
	}
	eventManager.GetInstance().EmitEvent("status", currentStatus)

	label := Label(status)
	entry := activity.Record(activity.Entry{
		Severity:    activity.SeverityStatus,
		Summary:     "Status: " + label,
		Detail:      message,
		StatusID:    int(status),
		StatusLabel: label,
	})
	eventManager.GetInstance().EmitEvent("activity.appended", entry)
}

func Get() StatusEvent {
	return currentStatus
}
