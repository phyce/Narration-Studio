package response

import (
	"errors"
	"fmt"
	"nstudio/app/common/eventManager"
)

type Data struct {
	Summary  string `json:"summary"`
	Detail   string `json:"detail"`
	Severity string `json:"severity"`
	Life     uint   `json:"life"`
}

var notificationEnabled = true

func Initialize() {
	eventManager.GetInstance().SubscribeToEvent("notification_enabled", func(data interface{}) {
		if enabled, ok := data.(bool); ok {
			notificationEnabled = enabled
		} else {
			Error(Data{
				Summary: fmt.Sprint(data) + " is not a valid value for notification_enabled",
			})
		}
	})
}

// TODO enable/disable logging
func Debug(data Data) {
	data.Severity = "info"
	data.Life = 2500
	//TODO toggle logging if debug mode on/off
	emitEvent("notification", data, true)
}

func Info(data Data) error {
	data.Severity = "info"
	data.Life = 10000
	emitEvent("notification", data, false)
	return nil
}

func Success(data Data) error {
	data.Severity = "success"
	data.Life = 3500
	emitEvent("notification", data, false)
	return nil
}

func Warning(data Data) error {
	data.Severity = "warning"
	data.Life = 5500
	emitEvent("notification", data, false)
	return nil
}

func Error(data Data) error {
	data.Severity = "error"
	data.Life = 25000
	emitEvent("notification", data, true)
	return errors.New(data.Summary)
}

func emitEvent(name string, data Data, log bool) {
	fmt.Println(data.Severity+": ", data.Summary, data.Detail)
	if notificationEnabled {
		eventManager.GetInstance().EmitEvent(name, data)
	}
}
