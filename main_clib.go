//go:build clib

package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"nstudio/app/common/audio"
	"nstudio/app/common/util"
	"nstudio/app/config"
	"nstudio/app/tts"
	ttsEngine "nstudio/app/tts/engine"
	"nstudio/app/tts/modelManager"
	"nstudio/app/tts/profile"
	"sync"
	"unsafe"
)

// ---------------------------------------------------------------------------
// Internal state
// ---------------------------------------------------------------------------

var (
	initialized bool
	initMu      sync.Mutex

	lastErrMu   sync.Mutex
	lastErrCode int
	lastErrMsg  string
)

func setLastError(code int, msg string) {
	lastErrMu.Lock()
	lastErrCode = code
	lastErrMsg = msg
	lastErrMu.Unlock()
}

func checkInit() bool {
	if !initialized {
		setLastError(-1, "NStudioInit has not been called")
		return false
	}
	return true
}

// returnJSON marshals v to JSON, copies to C-heap, writes to *out.
func returnJSON(v interface{}, out **C.char) C.int {
	data, err := json.Marshal(v)
	if err != nil {
		setLastError(-99, fmt.Sprintf("JSON marshal error: %v", err))
		return -99
	}
	*out = C.CString(string(data))
	return 0
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// main is required by c-shared build mode but never called.
func main() {}

//export NStudioInit
func NStudioInit(configJSON *C.char) C.int {
	initMu.Lock()
	defer initMu.Unlock()

	if initialized {
		return 0
	}

	defer func() {
		if r := recover(); r != nil {
			setLastError(-99, fmt.Sprintf("panic in NStudioInit: %v", r))
		}
	}()

	// Parse optional init config
	type initOpts struct {
		ModelsDirectory string `json:"modelsDirectory"`
		EspeakDataDir   string `json:"espeakDataDir"`
		ConfigFile      string `json:"configFile"`
		EnableCache     *bool  `json:"enableCache"`
		CacheDirectory  string `json:"cacheDirectory"`
	}

	var opts initOpts
	if configJSON != nil {
		goStr := C.GoString(configJSON)
		if goStr != "" {
			json.Unmarshal([]byte(goStr), &opts)
		}
	}

	// Initialize core systems
	if err := initializeApp(opts.ConfigFile); err != nil {
		setLastError(-6, fmt.Sprintf("initializeApp failed: %v", err))
		return -6
	}

	// Apply overrides from init options
	if opts.ModelsDirectory != "" {
		config.SetValueToPath("engine.local.piper.modelsDirectory", fmt.Sprintf("%q", opts.ModelsDirectory))
	}
	if opts.EspeakDataDir != "" {
		config.SetValueToPath("engine.local.piper.espeakDataDir", fmt.Sprintf("%q", opts.EspeakDataDir))
	}

	// Initialize model manager (no speaker output in DLL mode)
	modelManager.Initialize(false)

	// Register all engines
	registerEngines()

	initialized = true
	return 0
}

//export NStudioShutdown
func NStudioShutdown() {
	initMu.Lock()
	defer initMu.Unlock()
	initialized = false
}

//export NStudioFree
func NStudioFree(ptr unsafe.Pointer) {
	if ptr != nil {
		C.free(ptr)
	}
}

// ---------------------------------------------------------------------------
// TTS Generation
// ---------------------------------------------------------------------------

//export NStudioGenerate
func NStudioGenerate(requestJSON *C.char, outData **C.char, outLen *C.int, outMeta **C.char) (errCode C.int) {
	defer func() {
		if r := recover(); r != nil {
			setLastError(-99, fmt.Sprintf("panic: %v", r))
			errCode = -99
		}
	}()

	if !checkInit() {
		return -1
	}

	type genRequest struct {
		Engine string `json:"engine"`
		Model  string `json:"model"`
		Voice  string `json:"voice"`
		Text   string `json:"text"`
		Format string `json:"format"`
	}

	var req genRequest
	if err := json.Unmarshal([]byte(C.GoString(requestJSON)), &req); err != nil {
		setLastError(-2, fmt.Sprintf("invalid request JSON: %v", err))
		return -2
	}

	if req.Engine == "" || req.Model == "" || req.Voice == "" || req.Text == "" {
		setLastError(-2, "engine, model, voice, and text are required")
		return -2
	}

	if req.Format == "" {
		req.Format = "wav"
	}

	voice := &util.CharacterVoice{
		Name:   "nstudio",
		Engine: req.Engine,
		Model:  req.Model,
		Voice:  req.Voice,
	}

	audioObj, err := tts.GenerateAudio(voice, req.Text)
	if err != nil {
		setLastError(-4, fmt.Sprintf("generation failed: %v", err))
		return -4
	}

	outputBytes, err := audioObj.ToFormat(req.Format)
	if err != nil {
		setLastError(-4, fmt.Sprintf("format conversion failed: %v", err))
		return -4
	}

	// Copy audio data to C heap
	*outLen = C.int(len(outputBytes))
	*outData = (*C.char)(C.CBytes(outputBytes))

	// Build and return metadata
	meta := struct {
		SampleRate int    `json:"sampleRate"`
		Channels   int    `json:"channels"`
		BitDepth   int    `json:"bitDepth"`
		Format     string `json:"format"`
	}{
		SampleRate: audioObj.Metadata.SampleRate,
		Channels:   audioObj.Metadata.Channels,
		BitDepth:   audioObj.Metadata.BitDepth,
		Format:     req.Format,
	}

	metaJSON, _ := json.Marshal(meta)
	*outMeta = C.CString(string(metaJSON))

	return 0
}

//export NStudioGenerateForProfile
func NStudioGenerateForProfile(requestJSON *C.char, outData **C.char, outLen *C.int, outMeta **C.char) (errCode C.int) {
	defer func() {
		if r := recover(); r != nil {
			setLastError(-99, fmt.Sprintf("panic: %v", r))
			errCode = -99
		}
	}()

	if !checkInit() {
		return -1
	}

	type profileRequest struct {
		Profile   string `json:"profile"`
		Character string `json:"character"`
		Text      string `json:"text"`
		Format    string `json:"format"`
	}

	var req profileRequest
	if err := json.Unmarshal([]byte(C.GoString(requestJSON)), &req); err != nil {
		setLastError(-2, fmt.Sprintf("invalid request JSON: %v", err))
		return -2
	}

	if req.Profile == "" || req.Character == "" || req.Text == "" {
		setLastError(-2, "profile, character, and text are required")
		return -2
	}

	if req.Format == "" {
		req.Format = "wav"
	}

	manager := profile.GetManager()
	voice, err := manager.GetOrAllocateVoice(req.Profile, req.Character)
	if err != nil {
		setLastError(-5, fmt.Sprintf("profile voice allocation failed: %v", err))
		return -5
	}

	audioObj, err := tts.GenerateAudio(voice, req.Text)
	if err != nil {
		setLastError(-4, fmt.Sprintf("generation failed: %v", err))
		return -4
	}

	outputBytes, err := audioObj.ToFormat(req.Format)
	if err != nil {
		setLastError(-4, fmt.Sprintf("format conversion failed: %v", err))
		return -4
	}

	*outLen = C.int(len(outputBytes))
	*outData = (*C.char)(C.CBytes(outputBytes))

	meta := struct {
		SampleRate int    `json:"sampleRate"`
		Channels   int    `json:"channels"`
		BitDepth   int    `json:"bitDepth"`
		Format     string `json:"format"`
	}{
		SampleRate: audioObj.Metadata.SampleRate,
		Channels:   audioObj.Metadata.Channels,
		BitDepth:   audioObj.Metadata.BitDepth,
		Format:     req.Format,
	}

	metaJSON, _ := json.Marshal(meta)
	*outMeta = C.CString(string(metaJSON))

	return 0
}

// ---------------------------------------------------------------------------
// Engine / Voice Discovery
// ---------------------------------------------------------------------------

//export NStudioGetEngines
func NStudioGetEngines(outJSON **C.char) C.int {
	if !checkInit() {
		return -1
	}

	engines := modelManager.GetAllEngines()

	type engineOut struct {
		ID     string      `json:"id"`
		Name   string      `json:"name"`
		Type   string      `json:"type"`
		Tags   []string    `json:"tags"`
		Models interface{} `json:"models"`
	}

	var result []engineOut
	for _, eng := range engines {
		eType := "local"
		if eng.Type == 1 {
			eType = "api"
		}
		tags := eng.Tags
		if tags == nil {
			tags = []string{}
		}
		result = append(result, engineOut{
			ID:     eng.ID,
			Name:   eng.Name,
			Type:   eType,
			Tags:   tags,
			Models: eng.Models,
		})
	}

	if result == nil {
		result = []engineOut{}
	}

	return returnJSON(result, outJSON)
}

//export NStudioGetVoices
func NStudioGetVoices(engineID *C.char, modelID *C.char, outJSON **C.char) C.int {
	if !checkInit() {
		return -1
	}

	voices, err := modelManager.GetModelVoices(C.GoString(engineID), C.GoString(modelID))
	if err != nil {
		setLastError(-3, fmt.Sprintf("get voices failed: %v", err))
		return -3
	}

	if voices == nil {
		voices = []ttsEngine.Voice{}
	}

	return returnJSON(voices, outJSON)
}

//export NStudioGetAllVoices
func NStudioGetAllVoices(outJSON **C.char) C.int {
	if !checkInit() {
		return -1
	}

	engines := modelManager.GetAllEngines()

	type voiceEntry struct {
		Engine  string `json:"engine"`
		Model   string `json:"model"`
		VoiceID string `json:"voiceID"`
		Name    string `json:"name"`
		Gender  string `json:"gender"`
	}

	var result []voiceEntry
	for _, eng := range engines {
		for modelID := range eng.Models {
			voices, err := modelManager.GetModelVoices(eng.ID, modelID)
			if err != nil {
				continue
			}
			for _, v := range voices {
				result = append(result, voiceEntry{
					Engine:  eng.ID,
					Model:   modelID,
					VoiceID: v.ID,
					Name:    v.Name,
					Gender:  v.Gender,
				})
			}
		}
	}

	if result == nil {
		result = []voiceEntry{}
	}

	return returnJSON(result, outJSON)
}

// ---------------------------------------------------------------------------
// Configuration & Settings Schema
// ---------------------------------------------------------------------------

//export NStudioGetConfigSchema
func NStudioGetConfigSchema(outJSON **C.char) C.int {
	if !checkInit() {
		return -1
	}

	schema, err := config.GetConfigSchema()
	if err != nil {
		setLastError(-6, fmt.Sprintf("get config schema failed: %v", err))
		return -6
	}

	return returnJSON(schema, outJSON)
}

//export NStudioGetConfig
func NStudioGetConfig(outJSON **C.char) C.int {
	if !checkInit() {
		return -1
	}

	cfg := config.Get()
	return returnJSON(cfg, outJSON)
}

//export NStudioSetConfig
func NStudioSetConfig(configJSON *C.char) C.int {
	if !checkInit() {
		return -1
	}

	var newConfig config.Base
	if err := json.Unmarshal([]byte(C.GoString(configJSON)), &newConfig); err != nil {
		setLastError(-2, fmt.Sprintf("invalid config JSON: %v", err))
		return -2
	}

	if err := config.Set(newConfig); err != nil {
		setLastError(-6, fmt.Sprintf("set config failed: %v", err))
		return -6
	}

	return 0
}

//export NStudioSetConfigValue
func NStudioSetConfigValue(path *C.char, valueJSON *C.char) C.int {
	if !checkInit() {
		return -1
	}

	err := config.SetValueToPath(C.GoString(path), C.GoString(valueJSON))
	if err != nil {
		setLastError(-6, fmt.Sprintf("set config value failed: %v", err))
		return -6
	}

	return 0
}

// ---------------------------------------------------------------------------
// Profiles
// ---------------------------------------------------------------------------

//export NStudioGetProfiles
func NStudioGetProfiles(outJSON **C.char) C.int {
	if !checkInit() {
		return -1
	}

	manager := profile.GetManager()
	profiles, err := manager.GetAllProfiles()
	if err != nil {
		setLastError(-5, fmt.Sprintf("get profiles failed: %v", err))
		return -5
	}

	return returnJSON(profiles, outJSON)
}

//export NStudioCreateProfile
func NStudioCreateProfile(requestJSON *C.char, outJSON **C.char) C.int {
	if !checkInit() {
		return -1
	}

	type createReq struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	var req createReq
	if err := json.Unmarshal([]byte(C.GoString(requestJSON)), &req); err != nil {
		setLastError(-2, fmt.Sprintf("invalid request JSON: %v", err))
		return -2
	}

	manager := profile.GetManager()
	prof, err := manager.CreateProfile(req.ID, req.Name, req.Description)
	if err != nil {
		setLastError(-5, fmt.Sprintf("create profile failed: %v", err))
		return -5
	}

	return returnJSON(prof, outJSON)
}

//export NStudioDeleteProfile
func NStudioDeleteProfile(profileID *C.char) C.int {
	if !checkInit() {
		return -1
	}

	manager := profile.GetManager()
	if err := manager.DeleteProfile(C.GoString(profileID)); err != nil {
		setLastError(-5, fmt.Sprintf("delete profile failed: %v", err))
		return -5
	}

	return 0
}

//export NStudioGetProfileVoices
func NStudioGetProfileVoices(profileID *C.char, outJSON **C.char) C.int {
	if !checkInit() {
		return -1
	}

	manager := profile.GetManager()
	prof, err := manager.GetProfile(C.GoString(profileID))
	if err != nil {
		setLastError(-5, fmt.Sprintf("get profile failed: %v", err))
		return -5
	}

	return returnJSON(prof.Voices, outJSON)
}

//export NStudioSetProfileVoice
func NStudioSetProfileVoice(profileID *C.char, character *C.char, voiceJSON *C.char) C.int {
	if !checkInit() {
		return -1
	}

	var voice util.CharacterVoice
	if err := json.Unmarshal([]byte(C.GoString(voiceJSON)), &voice); err != nil {
		setLastError(-2, fmt.Sprintf("invalid voice JSON: %v", err))
		return -2
	}

	manager := profile.GetManager()
	prof, err := manager.GetProfile(C.GoString(profileID))
	if err != nil {
		setLastError(-5, fmt.Sprintf("get profile failed: %v", err))
		return -5
	}

	charName := C.GoString(character)
	if prof.Voices == nil {
		prof.Voices = make(map[string]*util.CharacterVoice)
	}
	prof.Voices[charName] = &voice

	if err := manager.SaveProfile(prof); err != nil {
		setLastError(-5, fmt.Sprintf("save profile failed: %v", err))
		return -5
	}

	return 0
}

// ---------------------------------------------------------------------------
// Model Management
// ---------------------------------------------------------------------------

//export NStudioReloadModels
func NStudioReloadModels() C.int {
	if !checkInit() {
		return -1
	}

	if err := modelManager.ReloadModels(); err != nil {
		setLastError(-3, fmt.Sprintf("reload models failed: %v", err))
		return -3
	}

	return 0
}

//export NStudioGetModelToggles
func NStudioGetModelToggles(outJSON **C.char) C.int {
	if !checkInit() {
		return -1
	}

	toggles := config.GetModelToggles()
	return returnJSON(toggles, outJSON)
}

//export NStudioSetModelToggle
func NStudioSetModelToggle(key *C.char, enabled C.int) C.int {
	if !checkInit() {
		return -1
	}

	goKey := C.GoString(key)
	value := "true"
	if enabled == 0 {
		value = "false"
	}

	err := config.SetValueToPath("modelToggles."+goKey, value)
	if err != nil {
		setLastError(-6, fmt.Sprintf("set model toggle failed: %v", err))
		return -6
	}

	return 0
}

// ---------------------------------------------------------------------------
// Error Handling
// ---------------------------------------------------------------------------

//export NStudioGetLastError
func NStudioGetLastError(outJSON **C.char) C.int {
	lastErrMu.Lock()
	code := lastErrCode
	msg := lastErrMsg
	lastErrMu.Unlock()

	if msg == "" {
		return -1
	}

	result := struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}{
		Code:    code,
		Message: msg,
	}

	return returnJSON(result, outJSON)
}

// Unused import guard -- audio is used for format constants only through tts.GenerateAudio
var _ = audio.FormatWAV
