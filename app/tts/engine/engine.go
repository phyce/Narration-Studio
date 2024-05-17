package engine

import (
	"fmt"
	"nstudio/app/tts/util"
	"reflect"
)

type EngineBase interface {
	Initialize() error
	Prepare() error
	Play(message util.CharacterMessage) error
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

func (engine *Engine[T]) Prepare() error {
	fmt.Println("Base prepare: %s")
	return nil
}
