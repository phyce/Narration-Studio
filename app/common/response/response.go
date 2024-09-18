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

// TODO enable/disable logging
func Debug(data Data) {
	fmt.Println(data.Summary, data.Detail)
	data.Severity = "info"
	data.Life = 2500
	emitEvent("notification", data)
}

func Info(data Data) error {
	data.Severity = "info"
	data.Life = 10000
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
	data.Life = 25000
	emitEvent("notification", data)
	return errors.New(data.Summary)
}

func emitEvent(name string, data Data) {
	eventManager.GetInstance().EmitEvent(name, data)
}
