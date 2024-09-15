package response

import (
	"errors"
	"nstudio/app/common/eventManager"
)

type Data struct {
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Detail   string `json:"detail"`
	Life     uint   `json:"life"`
}

// TODO enable/disable logging

func Info(data Data) error {
	data.Severity = "info"
	data.Life = 3000
	emitEvent("notification", data)
	return nil
}

func Success(data Data) error {
	data.Severity = "success"
	data.Life = 3500
	emitEvent("notification", data)
	return nil
}

func Warning(data Data) error {
	data.Severity = "warning"
	data.Life = 5500
	emitEvent("notification", data)
	return nil
}

func Error(data Data) error {
	data.Severity = "error"
	data.Life = 0
	emitEvent("notification", data)
	return errors.New(data.Summary)
}

func emitEvent(name string, data Data) {
	eventManager.GetInstance().EmitEvent(name, data)
}
