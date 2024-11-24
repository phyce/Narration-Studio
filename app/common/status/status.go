package status

import (
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

func Set(status ID, message string) {
	currentStatus = StatusEvent{
		Status:  status,
		Message: message,
	}
	eventManager.GetInstance().EmitEvent("status", currentStatus)
}

func Get() StatusEvent {
	return currentStatus
}
