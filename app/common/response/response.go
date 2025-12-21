package response

import (
	"errors"
	"fmt"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/util"
	"nstudio/app/config"

	"github.com/charmbracelet/log"
)

var notificationEnabled = true

func Initialize() {
	eventManager.GetInstance().SubscribeToEvent("notification.enabled", func(data interface{}) {
		if enabled, ok := data.(bool); ok {
			notificationEnabled = enabled
		} else {
			Error(util.MessageData{
				Summary: fmt.Sprint(data) + " is not a valid value for notification.enabled",
			})
		}
	})
}

// <editor-fold desc="Notification Actions">
func Debug(data util.MessageData) {
	if config.Debug() {
		data.Severity = "info"
		data.Life = 2500
		emitEvent("notification.send", data, true)
	}
}

func Info(data util.MessageData) error {
	data.Severity = "info"
	data.Life = 10000
	log.Info(data.Summary, data.Detail)

	emitEvent("notification.send", data, false)
	return nil
}

func Success(data util.MessageData) error {
	data.Severity = "success"
	data.Life = 3500
	log.Info(data.Summary, data.Detail)

	emitEvent("notification.send", data, false)
	return nil
}

func Warning(data util.MessageData) error {
	data.Severity = "warning"
	data.Life = 5500
	log.Warn(data.Summary, data.Detail)

	emitEvent("notification.send", data, false)
	return nil
}

func Error(data util.MessageData) error {
	data.Severity = "error"
	data.Life = 50000
	log.Error(data.Summary, data.Detail)

	emitEvent("notification.send", data, true)
	return trace(errors.New(data.Summary))
}

// Send user notification
func Alert(message string, err error) error {
	message = fmt.Sprintf(message, err)
	log.Error(message)
	log.Info(err)

	Error(util.MessageData{
		Summary: message,
		Detail:  err.Error(),
	})

	return trace(err)
}

// Send user notifcation only in debug mode
func Warn(message string, err error) error {
	message = fmt.Sprintf(message, err)
	log.Warn(message)
	log.Info(err)

	if config.Debug() {
		Warning(util.MessageData{
			Summary: message,
			Detail:  err.Error(),
		})
	}

	return trace(err)
}

func NewWarn(message string) error {
	err := fmt.Errorf(message)
	log.Warn(err)

	if config.Debug() {
		eventManager.GetInstance().EmitEvent("notification.send", message)
		Warning(util.MessageData{
			Summary: "Warning",
			Detail:  message,
		})
	}

	return trace(err)
}

// </editor-fold>

// <editor-fold desc="Logging">
func Err(err error) error {
	log.Error(err)
	return trace(err)
}

func LogInfo(message string) {
	log.Info(message)
}

// </editor-fold>
