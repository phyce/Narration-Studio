package eventManager

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

/* Currently used Events
notification.send - send a notification to the user
notification.enabled - enable/disable notifications
app.refresh - reload current view
status - current progress
*/

type EventManager struct {
	mutex     sync.RWMutex
	callbacks map[string]func(data interface{})
	context   context.Context
}

var instance *EventManager
var once sync.Once

func GetInstance() *EventManager {
	once.Do(func() {
		instance = &EventManager{
			callbacks: make(map[string]func(data interface{})),
		}
	})
	return instance
}

func (manager *EventManager) Initialize(context context.Context) {
	manager.context = context
}

func (manager *EventManager) SubscribeToEvent(eventName string, callback func(data interface{})) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.callbacks[eventName] = callback
}

func (manager *EventManager) EmitEvent(eventName string, data interface{}) {
	if manager.context != nil {
		runtime.EventsEmit(manager.context, eventName, data)
	}
}

func (manager *EventManager) TriggerEvent(eventName string, data interface{}) {
	manager.mutex.RLock()
	callback, exists := manager.callbacks[eventName]
	manager.mutex.RUnlock()

	if exists && callback != nil {
		callback(data)
	}
}
