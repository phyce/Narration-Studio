//go:build clib

package modelManager

// initSpeaker is a no-op in DLL mode. The host application handles audio output.
func initSpeaker() {
}
