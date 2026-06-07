package response

import (
	"errors"
	"fmt"
	"nstudio/app/common/activity"
	"nstudio/app/common/eventManager"
	"nstudio/app/common/util"

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
	log.Debug(data.Summary, data.Detail)
	record(activity.SeverityInfo, data.Summary, data.Detail)
}

func Info(data util.MessageData) error {
	log.Info(data.Summary, data.Detail)
	record(activity.SeverityInfo, data.Summary, data.Detail)
	return nil
}

func Success(data util.MessageData) error {
	data.Severity = "success"
	data.Life = 3500
	log.Info(data.Summary, data.Detail)

	record(activity.SeveritySuccess, data.Summary, data.Detail)
	emitEvent("notification.send", data, false)
	return nil
}

func Warning(data util.MessageData) error {
	log.Warn(data.Summary, data.Detail)
	record(activity.SeverityWarning, data.Summary, data.Detail)
	return nil
}

func Error(data util.MessageData) error {
	data.Severity = "error"
	data.Life = 50000
	log.Error(data.Summary, data.Detail)

	record(activity.SeverityError, data.Summary, data.Detail)
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

func Warn(message string, err error) error {
	message = fmt.Sprintf(message, err)
	log.Warn(message)
	log.Info(err)

	record(activity.SeverityWarning, message, err.Error())
	return trace(err)
}

func NewWarn(message string) error {
	err := errors.New(message)
	log.Warn(err)

	record(activity.SeverityWarning, message, "")
	return trace(err)
}

// </editor-fold>

// <editor-fold desc="Logging">
func Err(err error) error {
	log.Error(err)
	record(activity.SeverityError, err.Error(), "")
	return trace(err)
}

func LogInfo(message string) {
	log.Info(message)
	record(activity.SeverityInfo, message, "")
}

// </editor-fold>
