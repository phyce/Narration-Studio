//go:build cli || clib

package eventManager

// EmitEvent is a no-op in headless modes (no Wails frontend to receive events).
func (manager *EventManager) EmitEvent(eventName string, data interface{}) {
}
