package fileIndex

import "time"

var index int
var timestamp string

func Reset() {
	index = -1
	timestamp = time.Now().Format("15-04-05")
}

func Get() int {
	index++
	return index
}

func Timestamp() string {
	return timestamp
}
