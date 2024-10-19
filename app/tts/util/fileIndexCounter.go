package util

import "time"

var index int
var timestamp string

func FileIndexReset() {
	index = -1
	timestamp = time.Now().Format("15-04-05")
}

func FileIndexGet() int {
	index++
	return index
}

func FileTimestampGet() string {
	return timestamp
}
