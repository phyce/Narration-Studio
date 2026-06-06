package eventManager

import (
	"sync"
)

type EventManager struct {
	mutex     sync.RWMutex
	callbacks map[string]func(data interface{})
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

func (manager *EventManager) SubscribeToEvent(eventName string, callback func(data interface{})) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.callbacks[eventName] = callback
}

func (manager *EventManager) TriggerEvent(eventName string, data interface{}) {
	manager.mutex.RLock()
	callback, exists := manager.callbacks[eventName]
	manager.mutex.RUnlock()

	if exists && callback != nil {
		callback(data)
	}
}
