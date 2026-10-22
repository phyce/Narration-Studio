//go:build windows

package mssapi5

import (
	"nstudio/app/common/response"
	"nstudio/app/tts/engine"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

const (
	comResultAlreadyInitialized   = 0x00000001
	comErrorChangedThreadingMode  = 0x80010106
	sapiAudioFormat22kHz16BitMono = 22
	sapiStreamModeCreateForWrite  = 3
	sapiSpeakFlagsDefault         = 0
)

func comInit() error {
	err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err != nil {
		oleErr, ok := err.(*ole.OleError)
		if !ok {
			return response.Err("%w", err)
		}
		code := oleErr.Code()
		if code == comResultAlreadyInitialized {
			return nil
		}
		if code == comErrorChangedThreadingMode {
			err2 := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
			if err2 != nil {
				oleErr2, ok := err2.(*ole.OleError)
				if ok && oleErr2.Code() == comResultAlreadyInitialized {
					return nil
				}
				return response.Err("%w", err2)
			}
			return nil
		}
		return response.Err("%w", err)
	}
	return nil
}

func initCOM() error {
	return nil
}

func enumerateVoices() ([]engine.Voice, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := comInit(); err != nil {
		return nil, response.Err("COM init failed: %w", err)
	}

	unknown, err := oleutil.CreateObject("SAPI.SpVoice")
	if err != nil {
		return nil, response.Err("failed to create SAPI.SpVoice: %w", err)
	}
	defer unknown.Release()

	voice, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, response.Err("failed to get IDispatch for SpVoice: %w", err)
	}
	defer voice.Release()

	tokensResult, err := oleutil.CallMethod(voice, "GetVoices", "", "")
	if err != nil {
		return nil, response.Err("failed to call GetVoices: %w", err)
	}
	tokens := tokensResult.ToIDispatch()
	defer tokens.Release()

	countResult, err := oleutil.GetProperty(tokens, "Count")
	if err != nil {
		return nil, response.Err("failed to get voice count: %w", err)
	}
	count := int(countResult.Val)

	var voices []engine.Voice
	for i := 0; i < count; i++ {
		itemResult, err := oleutil.CallMethod(tokens, "Item", i)
		if err != nil {
			continue
		}
		token := itemResult.ToIDispatch()

		descResult, err := oleutil.CallMethod(token, "GetDescription", 0)
		if err != nil {
			token.Release()
			continue
		}
		name := descResult.ToString()

		idResult, err := oleutil.GetProperty(token, "Id")
		voiceID := ""
		if err == nil {
			fullID := idResult.ToString()
			parts := strings.Split(fullID, "\\")
			voiceID = parts[len(parts)-1]
		}

		gender := "Unknown"
		genderResult, err := oleutil.CallMethod(token, "GetAttribute", "Gender")
		if err == nil {
			genderStr := genderResult.ToString()
			switch strings.ToLower(genderStr) {
			case "male":
				gender = "Male"
			case "female":
				gender = "Female"
			}
		}

		if voiceID == "" {
			voiceID = name
		}

		voices = append(voices, engine.Voice{
			ID:     voiceID,
			Name:   name,
			Gender: gender,
		})

		token.Release()
	}

	return voices, nil
}

func synthesize(voiceID string, text string, rate int, volume int) ([]byte, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := comInit(); err != nil {
		return nil, response.Err("COM init failed: %w", err)
	}

	voiceUnknown, err := oleutil.CreateObject("SAPI.SpVoice")
	if err != nil {
		return nil, response.Err("failed to create SAPI.SpVoice: %w", err)
	}
	defer voiceUnknown.Release()

	voice, err := voiceUnknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, response.Err("failed to get IDispatch for SpVoice: %w", err)
	}
	defer voice.Release()

	tokensResult, err := oleutil.CallMethod(voice, "GetVoices", "", "")
	if err != nil {
		return nil, response.Err("failed to get voices: %w", err)
	}
	tokens := tokensResult.ToIDispatch()
	defer tokens.Release()

	countResult, err := oleutil.GetProperty(tokens, "Count")
	if err != nil {
		return nil, response.Err("failed to get voice count: %w", err)
	}
	count := int(countResult.Val)

	voiceSet := false
	for i := 0; i < count; i++ {
		itemResult, err := oleutil.CallMethod(tokens, "Item", i)
		if err != nil {
			continue
		}
		token := itemResult.ToIDispatch()

		idResult, err := oleutil.GetProperty(token, "Id")
		if err != nil {
			token.Release()
			continue
		}
		fullID := idResult.ToString()
		parts := strings.Split(fullID, "\\")
		shortID := parts[len(parts)-1]

		if shortID == voiceID {
			_, err = oleutil.PutPropertyRef(voice, "Voice", token)
			if err != nil {
				token.Release()
				return nil, response.Err("failed to set voice: %w", err)
			}
			voiceSet = true
			token.Release()
			break
		}
		token.Release()
	}

	if !voiceSet {
		return nil, response.Err("voice '%s' not found", voiceID)
	}

	if rate != 0 {
		oleutil.PutProperty(voice, "Rate", rate)
	}
	if volume > 0 {
		oleutil.PutProperty(voice, "Volume", volume)
	}

	streamUnknown, err := oleutil.CreateObject("SAPI.SpFileStream")
	if err != nil {
		return nil, response.Err("failed to create SAPI.SpFileStream: %w", err)
	}
	defer streamUnknown.Release()

	stream, err := streamUnknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, response.Err("failed to get IDispatch for SpFileStream: %w", err)
	}
	defer stream.Release()

	tempDir, err := os.MkdirTemp("", "mssapi5-*")
	if err != nil {
		return nil, response.Err("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	tempFile := filepath.Join(tempDir, "output.wav")

	formatUnknown, err := oleutil.CreateObject("SAPI.SpAudioFormat")
	if err != nil {
		return nil, response.Err("failed to create SpAudioFormat: %w", err)
	}
	defer formatUnknown.Release()

	format, err := formatUnknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, response.Err("failed to get IDispatch for SpAudioFormat: %w", err)
	}
	defer format.Release()

	// SAFT22kHz16BitMono = 22
	oleutil.PutProperty(format, "Type", 22)

	oleutil.PutPropertyRef(stream, "Format", format)

	// SSFMCreateForWrite = 3
	_, err = oleutil.CallMethod(stream, "Open", tempFile, 3)
	if err != nil {
		return nil, response.Err("failed to open output stream: %w", err)
	}

	_, err = oleutil.PutPropertyRef(voice, "AudioOutputStream", stream)
	if err != nil {
		oleutil.CallMethod(stream, "Close")
		return nil, response.Err("failed to set audio output stream: %w", err)
	}

	// Speak synchronously (SVSFDefault = 0)
	_, err = oleutil.CallMethod(voice, "Speak", text, 0)
	if err != nil {
		oleutil.CallMethod(stream, "Close")
		return nil, response.Err("SAPI5 Speak failed: %w", err)
	}

	oleutil.CallMethod(stream, "Close")

	wavBytes, err := os.ReadFile(tempFile)
	if err != nil {
		return nil, response.Err("failed to read generated WAV: %w", err)
	}

	return wavBytes, nil
}
