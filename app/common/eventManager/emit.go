//go:build !cli && !clib

package eventManager

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

func (manager *EventManager) EmitEvent(eventName string, data interface{}) {
	if app := application.Get(); app != nil {
		app.Event.Emit(eventName, data)
	}
}
