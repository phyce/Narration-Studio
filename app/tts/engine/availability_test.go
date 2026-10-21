package engine_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"nstudio/app/tts/engine"
	"nstudio/app/tts/engine/enginetest"
)

func TestMain(m *testing.M) {
	os.Exit(enginetest.RunWithTemporaryConfig(m))
}

func TestCheckApiAnswersSendsTheKeyAndAcceptsAnAnswer(t *testing.T) {
	var receivedKey string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedKey = request.Header.Get("x-api-key")
		writer.Write([]byte(`{"data": []}`))
	}))
	defer server.Close()

	if err := engine.CheckApiAnswers(context.Background(), "Test", server.URL, map[string]string{"x-api-key": "secret"}); err != nil {
		t.Fatalf("CheckApiAnswers: %v", err)
	}
	if receivedKey != "secret" {
		t.Errorf("the service got key %q, want secret", receivedKey)
	}
}

func TestCheckApiAnswersExplainsRefusals(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		status   int
		body     string
		expected string
	}{
		{"OpenAI", http.StatusUnauthorized, `{"error": {"message": "Incorrect API key provided"}}`, "OpenAI refused the API key (401 Unauthorized): Incorrect API key provided"},
		{"ElevenLabs", http.StatusUnauthorized, `{"detail": {"status": "invalid_api_key", "message": "Invalid API key"}}`, "ElevenLabs refused the API key (401 Unauthorized): Invalid API key"},
		{"Gemini", http.StatusBadRequest, `{"error": {"code": 400, "message": "API key not valid. Please pass a valid API key."}}`, "Gemini answered 400 Bad Request: API key not valid. Please pass a valid API key."},
		{"Busy", http.StatusServiceUnavailable, "try later", "Busy answered 503 Service Unavailable"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(testCase.status)
				writer.Write([]byte(testCase.body))
			}))
			defer server.Close()

			err := engine.CheckApiAnswers(context.Background(), testCase.name, server.URL, nil)
			if err == nil || err.Error() != testCase.expected {
				t.Errorf("got %v, want %q", err, testCase.expected)
			}
		})
	}
}

func TestCheckApiAnswersReportsAServiceThatDoesntAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	serviceURL := server.URL
	server.Close()

	err := engine.CheckApiAnswers(context.Background(), "Gone", serviceURL, nil)
	if err == nil || !strings.HasPrefix(err.Error(), "Gone didn't answer: ") {
		t.Errorf("got %v, want a \"didn't answer\" error", err)
	}
}
