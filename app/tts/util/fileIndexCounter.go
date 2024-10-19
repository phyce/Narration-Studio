package util

var index int

func FileIndexReset() {
	index = -1
}

func FileIndexGet() int {
	index++
	return index
}
