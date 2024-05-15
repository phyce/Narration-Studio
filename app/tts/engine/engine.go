package engine

import (
	"fmt"
	"nstudio/app/tts/util"
	"reflect"
)

type EngineBase interface {
	Initialize()
	Play(message util.CharacterMessage)
}

type Engine[T EngineBase] struct {
	Name string
}

func (engine *Engine[T]) Initialize() error {
	tType := reflect.TypeOf((*T)(nil)).Elem()
	panic(fmt.Sprintf("Initialize for %s engine has not been defined", tType.Name()))
}

func (engine *Engine[T]) Play(message util.CharacterMessage) error {
	fmt.Println("Base play: %s", message.Text)
	return nil
}
