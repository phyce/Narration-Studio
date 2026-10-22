//go:build !cli && !clib

package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"nstudio/app/cache"
	"nstudio/app/config"
	serverhttp "nstudio/app/server/http"
	"nstudio/app/tts/engine"
	"nstudio/app/tts/modelManager"
	"nstudio/app/tts/profile"
)

func startServerTestInstance(t *testing.T) string {
	t.Helper()
	testServer := httptest.NewServer(serverhttp.NewEchoServer())
	t.Cleanup(testServer.Close)
	return testServer.URL
}

func serverRequest(t *testing.T, method, url, body string, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("build request %s %s: %v", method, url, err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	httpResponse, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer httpResponse.Body.Close()
	data, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		t.Fatalf("read response of %s %s: %v", method, url, err)
	}
	return httpResponse, data
}

func requireStatus(t *testing.T, httpResponse *http.Response, body []byte, expected int) {
	t.Helper()
	if httpResponse.StatusCode != expected {
		t.Fatalf("%s %s: status = %d, want %d (body: %.300s)",
			httpResponse.Request.Method, httpResponse.Request.URL.Path, httpResponse.StatusCode, expected, string(body))
	}
}

func assertStreamingWAV(t *testing.T, body []byte) {
	t.Helper()
	if len(body) <= 44 {
		t.Fatalf("WAV body too short: %d bytes", len(body))
	}
	if string(body[0:4]) != "RIFF" || string(body[8:12]) != "WAVE" {
		t.Fatalf("body is not a WAV (first bytes %q)", body[0:12])
	}
	if binary.LittleEndian.Uint32(body[4:8]) != 0xFFFFFFFF {
		t.Errorf("streaming WAV RIFF size = %08X, want FFFFFFFF", binary.LittleEndian.Uint32(body[4:8]))
	}
	if binary.LittleEndian.Uint32(body[40:44]) != 0xFFFFFFFF {
		t.Errorf("streaming WAV data size = %08X, want FFFFFFFF", binary.LittleEndian.Uint32(body[40:44]))
	}
	if (len(body)-44)%2 != 0 {
		t.Errorf("PCM payload has odd byte count %d", len(body)-44)
	}
}

func modelVoices(t *testing.T, baseURL, engineID, modelID string) []struct {
	VoiceID string `json:"voiceID"`
	Name    string `json:"name"`
} {
	t.Helper()
	httpResponse, body := serverRequest(t, http.MethodGet,
		fmt.Sprintf("%s/engines/%s/models/%s/voices", baseURL, engineID, modelID), "", nil)
	requireStatus(t, httpResponse, body, http.StatusOK)
	var voices []struct {
		VoiceID string `json:"voiceID"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(body, &voices); err != nil {
		t.Fatalf("parse voices for %s:%s: %v (%.200s)", engineID, modelID, err, string(body))
	}
	return voices
}

func synthesizeViaServer(t *testing.T, baseURL, engineID, modelID, voiceID, text string) []byte {
	t.Helper()
	httpResponse, body := serverRequest(t, http.MethodPost,
		fmt.Sprintf("%s/tts/%s/%s/%s", baseURL, engineID, modelID, voiceID),
		fmt.Sprintf(`{"text":%q}`, text), nil)
	requireStatus(t, httpResponse, body, http.StatusOK)
	return body
}

func readRealUserConfig(t *testing.T) config.Base {
	t.Helper()
	var base config.Base
	configDir, err := os.UserConfigDir()
	if err != nil {
		return base
	}
	data, err := os.ReadFile(filepath.Join(configDir, "Narration Studio", "config.json"))
	if err != nil {
		return base
	}
	_ = json.Unmarshal(data, &base)
	return base
}

func sortedModelIDs(models map[string]engine.Model) []string {
	ids := make([]string, 0, len(models))
	for id := range models {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func TestServerEndpoints(t *testing.T) {
	originalConfig := config.Get()
	working := config.Get()
	if working.ModelToggles == nil {
		working.ModelToggles = map[string]bool{}
	}
	working.ModelToggles["mssapi5:mssapi5"] = true
	if err := config.Set(working); err != nil {
		t.Fatalf("enable mssapi5 toggle: %v", err)
	}
	t.Cleanup(func() { _ = config.Set(originalConfig) })

	baseURL := startServerTestInstance(t)
	var voiceID string

	t.Run("Health", func(t *testing.T) {
		httpResponse, body := serverRequest(t, http.MethodGet, baseURL+"/health", "", nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
		if !strings.Contains(string(body), `"status":"healthy"`) {
			t.Errorf("health response missing healthy status: %.200s", string(body))
		}
	})

	t.Run("Info", func(t *testing.T) {
		httpResponse, body := serverRequest(t, http.MethodGet, baseURL+"/info", "", nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
		if !strings.Contains(string(body), "Narration Studio API") {
			t.Errorf("info response missing API name: %.200s", string(body))
		}
	})

	t.Run("Engines", func(t *testing.T) {
		httpResponse, body := serverRequest(t, http.MethodGet, baseURL+"/engines", "", nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
		if !strings.Contains(string(body), `"id":"mssapi5"`) {
			t.Errorf("engines response missing mssapi5: %.300s", string(body))
		}
	})

	t.Run("EngineModels", func(t *testing.T) {
		httpResponse, body := serverRequest(t, http.MethodGet, baseURL+"/engines/mssapi5/models", "", nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
		if !strings.Contains(string(body), `"id":"mssapi5"`) {
			t.Errorf("models response missing mssapi5 model: %.300s", string(body))
		}
	})

	t.Run("UnknownEngineModels", func(t *testing.T) {
		httpResponse, body := serverRequest(t, http.MethodGet, baseURL+"/engines/doesnotexist/models", "", nil)
		requireStatus(t, httpResponse, body, http.StatusNotFound)
	})

	t.Run("EngineModelVoices", func(t *testing.T) {
		voices := modelVoices(t, baseURL, "mssapi5", "mssapi5")
		if len(voices) == 0 {
			t.Fatal("no SAPI5 voices returned")
		}
		for index, voice := range voices {
			if voice.VoiceID == "" {
				t.Errorf("voice %d has empty voiceID", index)
			}
		}
		voiceID = voices[0].VoiceID
	})

	t.Run("VoiceTree", func(t *testing.T) {
		httpResponse, body := serverRequest(t, http.MethodGet, baseURL+"/voices", "", nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
	})

	t.Run("ProfileCRUD", func(t *testing.T) {
		httpResponse, body := serverRequest(t, http.MethodPost, baseURL+"/profiles",
			`{"id":"server-test-profile","name":"Server Test","description":"endpoint test"}`, nil)
		requireStatus(t, httpResponse, body, http.StatusCreated)

		httpResponse, body = serverRequest(t, http.MethodGet, baseURL+"/profiles", "", nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
		if !strings.Contains(string(body), "server-test-profile") {
			t.Errorf("profile list missing created profile: %.300s", string(body))
		}

		httpResponse, body = serverRequest(t, http.MethodGet, baseURL+"/profiles/server-test-profile", "", nil)
		requireStatus(t, httpResponse, body, http.StatusOK)

		httpResponse, body = serverRequest(t, http.MethodPost, baseURL+"/profiles/server-test-profile/voices/bob", `{}`, nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
		if !strings.Contains(string(body), `"engine"`) {
			t.Errorf("set character voice returned no allocation: %.300s", string(body))
		}

		httpResponse, body = serverRequest(t, http.MethodGet, baseURL+"/profiles/server-test-profile/voices/bob", "", nil)
		requireStatus(t, httpResponse, body, http.StatusOK)

		httpResponse, body = serverRequest(t, http.MethodGet, baseURL+"/profiles/server-test-profile/voices", "", nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
		if !strings.Contains(string(body), `"count":1`) {
			t.Errorf("profile voices count != 1: %.300s", string(body))
		}

		httpResponse, body = serverRequest(t, http.MethodDelete, baseURL+"/profiles/server-test-profile/voices/bob", "", nil)
		requireStatus(t, httpResponse, body, http.StatusOK)

		httpResponse, body = serverRequest(t, http.MethodGet, baseURL+"/profiles/server-test-profile/voices/bob", "", nil)
		requireStatus(t, httpResponse, body, http.StatusNotFound)

		httpResponse, body = serverRequest(t, http.MethodDelete, baseURL+"/profiles/server-test-profile", "", nil)
		requireStatus(t, httpResponse, body, http.StatusOK)

		httpResponse, body = serverRequest(t, http.MethodGet, baseURL+"/profiles/server-test-profile", "", nil)
		requireStatus(t, httpResponse, body, http.StatusNotFound)
	})

	t.Run("TTSValidation", func(t *testing.T) {
		cases := []struct {
			name   string
			url    string
			body   string
			status int
		}{
			{"MissingProfile", "/tts", `{"character":"x","text":"hi"}`, http.StatusBadRequest},
			{"MissingCharacter", "/tts", `{"profile":"default","text":"hi"}`, http.StatusBadRequest},
			{"MissingText", "/tts", `{"profile":"default","character":"x"}`, http.StatusBadRequest},
			{"TextTooLong", "/tts", fmt.Sprintf(`{"profile":"default","character":"x","text":%q}`, strings.Repeat("a", 10001)), http.StatusBadRequest},
			{"InvalidFormat", "/tts", `{"profile":"default","character":"x","text":"hi","options":{"audio":{"format":"xyz"}}}`, http.StatusBadRequest},
			{"SimpleMissingText", "/tts/mssapi5/mssapi5/anything", `{"text":"  "}`, http.StatusBadRequest},
			{"SimpleUnknownEngine", "/tts/doesnotexist/model/voice", `{"text":"hi"}`, http.StatusInternalServerError},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				httpResponse, body := serverRequest(t, http.MethodPost, baseURL+testCase.url, testCase.body, nil)
				requireStatus(t, httpResponse, body, testCase.status)
			})
		}
	})

	t.Run("SimpleTTSStreamsWAV", func(t *testing.T) {
		if voiceID == "" {
			t.Skip("no SAPI5 voice available")
		}
		httpResponse, body := serverRequest(t, http.MethodPost,
			fmt.Sprintf("%s/tts/mssapi5/mssapi5/%s", baseURL, voiceID),
			`{"text":"Server endpoint streaming test."}`, nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
		assertStreamingWAV(t, body)
		if len(httpResponse.TransferEncoding) == 0 || httpResponse.TransferEncoding[0] != "chunked" {
			t.Errorf("expected chunked transfer encoding, got %v", httpResponse.TransferEncoding)
		}
	})

	t.Run("SimpleTTSStreamsPCM", func(t *testing.T) {
		if voiceID == "" {
			t.Skip("no SAPI5 voice available")
		}
		for _, format := range []string{"pcm", "pcm_s16le"} {
			t.Run(format, func(t *testing.T) {
				httpResponse, body := serverRequest(t, http.MethodPost,
					fmt.Sprintf("%s/tts/mssapi5/mssapi5/%s", baseURL, voiceID),
					fmt.Sprintf(`{"text":"Simple endpoint raw PCM test.","options":{"format":%q}}`, format), nil)
				requireStatus(t, httpResponse, body, http.StatusOK)
				if len(body) == 0 || len(body)%2 != 0 {
					t.Fatalf("PCM body invalid: %d bytes", len(body))
				}
				if len(body) >= 4 && string(body[0:4]) == "RIFF" {
					t.Error("PCM response carries a WAV header")
				}
				if contentType := httpResponse.Header.Get("Content-Type"); contentType != "audio/pcm" {
					t.Errorf("Content-Type = %q, want audio/pcm", contentType)
				}
				if disposition := httpResponse.Header.Get("Content-Disposition"); !strings.HasSuffix(disposition, `.pcm"`) {
					t.Errorf("Content-Disposition = %q, want a .pcm filename", disposition)
				}
			})
		}
	})

	t.Run("ProfileTTSStreamsPCM", func(t *testing.T) {
		httpResponse, body := serverRequest(t, http.MethodPost, baseURL+"/tts",
			`{"profile":"server-tts-test","character":"narrator","text":"Raw PCM streaming test.","options":{"audio":{"format":"pcm"}}}`, nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
		if len(body) == 0 || len(body)%2 != 0 {
			t.Fatalf("PCM body invalid: %d bytes", len(body))
		}
		if len(body) >= 4 && string(body[0:4]) == "RIFF" {
			t.Error("PCM response carries a WAV header")
		}
	})

	t.Run("ProfileTTSBufferedTransform", func(t *testing.T) {
		httpResponse, body := serverRequest(t, http.MethodPost, baseURL+"/tts",
			`{"profile":"server-tts-test","character":"narrator","text":"Buffered transform test.","options":{"audio":{"format":"wav","sample_rate":32000}}}`, nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
		if len(body) <= 44 || string(body[0:4]) != "RIFF" {
			t.Fatalf("invalid WAV response (%d bytes)", len(body))
		}
		if riffSize := binary.LittleEndian.Uint32(body[4:8]); int(riffSize) != len(body)-8 {
			t.Errorf("buffered WAV RIFF size = %d, want %d", riffSize, len(body)-8)
		}
		if rate := binary.LittleEndian.Uint32(body[24:28]); rate != 32000 {
			t.Errorf("sample rate = %d, want 32000", rate)
		}
	})

	t.Run("Auth", func(t *testing.T) {
		authConfig := config.Get()
		authConfig.Settings.Server.Auth.Key = "test-key"
		authConfig.Settings.Server.Auth.AdminKey = "test-admin-key"
		if err := config.Set(authConfig); err != nil {
			t.Fatalf("set auth keys: %v", err)
		}
		defer func() {
			restored := config.Get()
			restored.Settings.Server.Auth.Key = originalConfig.Settings.Server.Auth.Key
			restored.Settings.Server.Auth.AdminKey = originalConfig.Settings.Server.Auth.AdminKey
			_ = config.Set(restored)
		}()

		httpResponse, body := serverRequest(t, http.MethodGet, baseURL+"/engines", "", nil)
		requireStatus(t, httpResponse, body, http.StatusUnauthorized)

		httpResponse, body = serverRequest(t, http.MethodGet, baseURL+"/engines", "", map[string]string{"Authorization": "Bearer test-key"})
		requireStatus(t, httpResponse, body, http.StatusOK)

		httpResponse, body = serverRequest(t, http.MethodGet, baseURL+"/config", "", map[string]string{"Authorization": "Bearer test-key"})
		requireStatus(t, httpResponse, body, http.StatusForbidden)

		httpResponse, body = serverRequest(t, http.MethodGet, baseURL+"/config", "", map[string]string{"Authorization": "Bearer test-admin-key"})
		requireStatus(t, httpResponse, body, http.StatusOK)
	})

	t.Run("AdminConfig", func(t *testing.T) {
		httpResponse, body := serverRequest(t, http.MethodGet, baseURL+"/config", "", nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
		if !strings.Contains(string(body), `"success":true`) {
			t.Errorf("config response missing success: %.200s", string(body))
		}

		httpResponse, body = serverRequest(t, http.MethodGet, baseURL+"/config/schema", "", nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
	})
}

func TestServerEngineMatrix(t *testing.T) {
	originalConfig := config.Get()
	realConfig := readRealUserConfig(t)
	working := config.Get()
	working.Engine.Local = realConfig.Engine.Local

	working.Engine.Local.Piper.UseExecutable = true
	if err := config.Set(working); err != nil {
		t.Fatalf("apply real local engine config: %v", err)
	}
	registerEngines()

	includePacks := os.Getenv("NSTUDIO_TEST_PACKS") == "1"
	type matrixModel struct {
		engineID string
		modelID  string
	}
	var matrix []matrixModel
	for _, registeredEngine := range modelManager.GetAllEngines() {
		if registeredEngine.Type == engine.Api {
			continue
		}
		isPythonPack := false
		for _, tag := range registeredEngine.Tags {
			if tag == "python" {
				isPythonPack = true
				break
			}
		}
		if isPythonPack && !includePacks {
			t.Logf("skipping python pack engine %s (set NSTUDIO_TEST_PACKS=1 to include)", registeredEngine.ID)
			continue
		}
		for _, modelID := range sortedModelIDs(registeredEngine.Models) {
			matrix = append(matrix, matrixModel{registeredEngine.ID, modelID})
		}
	}

	toggled := config.Get()
	if toggled.ModelToggles == nil {
		toggled.ModelToggles = map[string]bool{}
	}
	for _, entry := range matrix {
		toggled.ModelToggles[entry.engineID+":"+entry.modelID] = true
	}
	if err := config.Set(toggled); err != nil {
		t.Fatalf("enable matrix model toggles: %v", err)
	}
	if err := modelManager.RefreshModels(); err != nil {
		t.Fatalf("start matrix models: %v", err)
	}

	t.Cleanup(func() {
		stopping := config.Get()
		for _, entry := range matrix {
			stopping.ModelToggles[entry.engineID+":"+entry.modelID] = false
		}
		_ = config.Set(stopping)
		_ = modelManager.RefreshModels()
		_ = config.Set(originalConfig)
		registerEngines()
	})

	baseURL := startServerTestInstance(t)

	for _, entry := range matrix {
		t.Run(entry.engineID+"/"+entry.modelID, func(t *testing.T) {
			voices := modelVoices(t, baseURL, entry.engineID, entry.modelID)
			if len(voices) == 0 {
				t.Fatalf("%s:%s has no voices", entry.engineID, entry.modelID)
			}
			body := synthesizeViaServer(t, baseURL, entry.engineID, entry.modelID, voices[0].VoiceID, "Server engine matrix test.")
			assertStreamingWAV(t, body)
		})
	}

	if len(matrix) == 0 {
		t.Fatal("no local engine models were available to test")
	}
}

func TestServerStreamingCache(t *testing.T) {
	const profileID = "server-cache-test"
	const character = "narrator"

	originalConfig := config.Get()
	working := config.Get()
	if working.ModelToggles == nil {
		working.ModelToggles = map[string]bool{}
	}
	working.ModelToggles["mssapi5:mssapi5"] = true
	working.Settings.AudioCache.Enabled = true
	working.Settings.AudioCache.Location = t.TempDir()
	if err := config.Set(working); err != nil {
		t.Fatalf("enable audio cache: %v", err)
	}
	if err := cache.Initialize(); err != nil {
		t.Fatalf("initialize cache: %v", err)
	}
	t.Cleanup(func() {
		_ = config.Set(originalConfig)
		_ = cache.Initialize()
	})

	cacheManager := cache.GetManager()
	if !cacheManager.IsEnabled() {
		t.Skip("audio cache could not be enabled in this environment")
	}

	profileManager := profile.GetManager()
	_ = profileManager.DeleteProfile(profileID)
	testProfile, err := profileManager.CreateProfile(profileID, "Server Cache Test", "streaming cache test")
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	cacheEnabled := true
	testProfile.SetSettings(&profile.ProfileSettings{CacheEnabled: &cacheEnabled})
	if err := profileManager.SaveProfile(testProfile); err != nil {
		t.Fatalf("save profile: %v", err)
	}
	t.Cleanup(func() { _ = profileManager.DeleteProfile(profileID) })

	baseURL := startServerTestInstance(t)
	const cachedText = "Streaming cache store test."
	var streamedPayload int

	t.Run("StoresCompletedStream", func(t *testing.T) {
		httpResponse, body := serverRequest(t, http.MethodPost, baseURL+"/tts",
			fmt.Sprintf(`{"profile":%q,"character":%q,"text":%q}`, profileID, character, cachedText), nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
		assertStreamingWAV(t, body)
		streamedPayload = len(body) - 44

		cached, found := cacheManager.GetCachedAudio(profileID, character, cachedText)
		if !found {
			t.Fatal("completed stream was not written to the cache")
		}
		if len(cached) <= 44 || string(cached[0:4]) != "RIFF" || string(cached[8:12]) != "WAVE" {
			t.Fatalf("cache entry is not a WAV (%d bytes)", len(cached))
		}
		if riffSize := binary.LittleEndian.Uint32(cached[4:8]); int(riffSize) != len(cached)-8 {
			t.Errorf("cached RIFF size = %d, want %d", riffSize, len(cached)-8)
		}
		if got := len(cached) - 44; got != streamedPayload {
			t.Errorf("cached %d PCM bytes, client received %d", got, streamedPayload)
		}
	})

	t.Run("ServesCacheHitBuffered", func(t *testing.T) {
		if streamedPayload == 0 {
			t.Skip("nothing was cached by the previous subtest")
		}
		httpResponse, body := serverRequest(t, http.MethodPost, baseURL+"/tts",
			fmt.Sprintf(`{"profile":%q,"character":%q,"text":%q}`, profileID, character, cachedText), nil)
		requireStatus(t, httpResponse, body, http.StatusOK)
		if len(body) <= 44 || string(body[0:4]) != "RIFF" {
			t.Fatalf("invalid WAV response (%d bytes)", len(body))
		}
		if riffSize := binary.LittleEndian.Uint32(body[4:8]); int(riffSize) != len(body)-8 {
			t.Errorf("cache hit returned streaming RIFF size %08X, want the buffered real size %d", riffSize, len(body)-8)
		}
		if got := len(body) - 44; got != streamedPayload {
			t.Errorf("cache hit returned %d PCM bytes, want %d", got, streamedPayload)
		}
	})

	t.Run("SkipsCacheOnClientDisconnect", func(t *testing.T) {
		abandonedText := strings.TrimSpace(strings.Repeat("This sentence keeps the synthesizer busy for a good long while. ", 12))

		requestContext, cancelRequest := context.WithCancel(context.Background())
		request, err := http.NewRequestWithContext(requestContext, http.MethodPost, baseURL+"/tts",
			strings.NewReader(fmt.Sprintf(`{"profile":%q,"character":%q,"text":%q}`, profileID, character, abandonedText)))
		if err != nil {
			cancelRequest()
			t.Fatalf("build request: %v", err)
		}
		request.Header.Set("Content-Type", "application/json")

		httpResponse, err := http.DefaultClient.Do(request)
		if err != nil {
			cancelRequest()
			t.Fatalf("start streaming request: %v", err)
		}
		prefix := make([]byte, 8*1024)
		_, readErr := io.ReadFull(httpResponse.Body, prefix)
		cancelRequest()
		_ = httpResponse.Body.Close()
		if readErr != nil {
			t.Fatalf("read stream prefix: %v", readErr)
		}

		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if cached, found := cacheManager.GetCachedAudio(profileID, character, abandonedText); found {
				t.Fatalf("abandoned stream was cached as complete (%d bytes)", len(cached))
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
}

func TestServerAPIEngines(t *testing.T) {
	originalConfig := config.Get()
	realConfig := readRealUserConfig(t)
	resolveKey := func(environmentVariable, fallback string) string {
		if value := os.Getenv(environmentVariable); value != "" {
			return value
		}
		return fallback
	}

	working := config.Get()
	working.Engine.Api.OpenAI.ApiKey = resolveKey("OPENAI_API_KEY", realConfig.Engine.Api.OpenAI.ApiKey)
	working.Engine.Api.ElevenLabs.ApiKey = resolveKey("ELEVENLABS_API_KEY", realConfig.Engine.Api.ElevenLabs.ApiKey)
	working.Engine.Api.Google.ApiKey = resolveKey("GOOGLE_API_KEY", realConfig.Engine.Api.Google.ApiKey)
	working.Engine.Api.Gemini.ApiKey = resolveKey("GEMINI_API_KEY", realConfig.Engine.Api.Gemini.ApiKey)
	if err := config.Set(working); err != nil {
		t.Fatalf("apply API keys: %v", err)
	}
	registerEngines()
	t.Cleanup(func() {
		_ = config.Set(originalConfig)
		registerEngines()
	})

	baseURL := startServerTestInstance(t)

	preferredModels := map[string]string{
		"openai":     "tts-1",
		"google":     "wavenet",
		"elevenlabs": "eleven_multilingual_v2",
	}

	testedEngines := 0
	for _, registeredEngine := range modelManager.GetAllEngines() {
		if registeredEngine.Type != engine.Api || len(registeredEngine.Models) == 0 {
			continue
		}
		modelID := preferredModels[registeredEngine.ID]
		if _, exists := registeredEngine.Models[modelID]; !exists {
			modelID = sortedModelIDs(registeredEngine.Models)[0]
		}

		testedEngines++
		t.Run(registeredEngine.ID+"/"+modelID, func(t *testing.T) {
			voices := modelVoices(t, baseURL, registeredEngine.ID, modelID)
			if len(voices) == 0 {
				t.Fatalf("%s:%s has no voices", registeredEngine.ID, modelID)
			}
			body := synthesizeViaServer(t, baseURL, registeredEngine.ID, modelID, voices[0].VoiceID, "Server API engine test.")
			assertStreamingWAV(t, body)
		})
	}

	if testedEngines == 0 {
		t.Skip("no API engine keys configured (set *_API_KEY or fill the real config)")
	}
}
