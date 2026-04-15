package Engines

type Engine string

const (
	Piper   Engine = "piper"
	MsSapi4 Engine = "mssapi4"
	MsSapi5 Engine = "mssapi5"

	OpenAI     Engine = "openai"
	ElevenLabs Engine = "elevenlabs"
	Google     Engine = "google"
	Gemini     Engine = "gemini"
)
