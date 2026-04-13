//go:build !cli && !clib

package eventManager

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (manager *EventManager) EmitEvent(eventName string, data interface{}) {
	if manager.context != nil {
		runtime.EventsEmit(manager.context, eventName, data)
	}
}
