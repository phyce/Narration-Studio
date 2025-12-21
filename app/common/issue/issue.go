package issue

import (
	"github.com/ncruces/zenity"
)

func Panic(message string, err error) {
	//TODO: Add some form of crash dump/log

	showErrorDialog(
		message,
		err.Error(),
	)
	panic(err)
}

func showErrorDialog(title, message string) {
	err := zenity.Error(message, zenity.Title(title))
	if err != nil {
		panic(err)
	}
}
