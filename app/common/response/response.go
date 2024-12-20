package response

import (
	"errors"
	"fmt"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/issue"
	"nstudio/app/config"
)

type Data struct {
	Summary  string `json:"summary"`
	Detail   string `json:"detail"`
	Severity string `json:"severity"`
	Life     uint   `json:"life"`
}

var notificationEnabled = true

func Initialize() {
	eventManager.GetInstance().SubscribeToEvent("notification.enabled", func(data interface{}) {
		if enabled, ok := data.(bool); ok {
			notificationEnabled = enabled
		} else {
			Error(Data{
				Summary: fmt.Sprint(data) + " is not a valid value for notification.enabled",
			})
		}
	})
}

func Debug(data Data) {
	if config.Debug() {
		data.Severity = "info"
		data.Life = 2500
		emitEvent("notification.send", data, true)
	}
}

func Info(data Data) error {
	data.Severity = "info"
	data.Life = 10000
	emitEvent("notification.send", data, false)
	return nil
}

func Success(data Data) error {
	data.Severity = "success"
	data.Life = 3500
	emitEvent("notification.send", data, false)
	return nil
}

func Warning(data Data) error {
	data.Severity = "warning"
	data.Life = 5500
	emitEvent("notification.send", data, false)
	return nil
}

func Error(data Data) error {
	data.Severity = "error"
	data.Life = 50000
	emitEvent("notification.send", data, true)
	return issue.Trace(errors.New(data.Summary))
}

func emitEvent(name string, data Data, log bool) {
	if config.Debug() {
		fmt.Println(fmt.Sprintf("event: %s - %s - %s ", data.Severity, data.Summary, data.Detail))
	}
	if notificationEnabled {
		eventManager.GetInstance().EmitEvent(name, data)
	}
}
