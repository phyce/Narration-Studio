package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"nstudio/app/common/response"
	"strings"
	"time"
)

var ErrApiKeyNotSet = errors.New("no API key set")

type AvailabilityChecker interface {
	CheckAvailability(ctx context.Context) error
}

const availabilityCheckTimeout = 10 * time.Second

const longestAnswerMessage = 200

var availabilityClient = &http.Client{}

func CheckApiAnswers(ctx context.Context, serviceName, url string, headers map[string]string) error {
	ctx, cancel := context.WithTimeout(ctx, availabilityCheckTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return response.Err("check %s: %w", serviceName, err)
	}
	for headerName, headerValue := range headers {
		request.Header.Set(headerName, headerValue)
	}

	answer, err := availabilityClient.Do(request)
	if err != nil {
		return response.Err("%s didn't answer: %w", serviceName, err)
	}
	defer answer.Body.Close()
	answerBody, _ := io.ReadAll(io.LimitReader(answer.Body, 64*1024))

	if answer.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	message := answerMessage(answerBody)
	if answer.StatusCode == http.StatusUnauthorized || answer.StatusCode == http.StatusForbidden {
		return response.Err("%s refused the API key (%s)%s", serviceName, answer.Status, message)
	}
	return response.Err("%s answered %s%s", serviceName, answer.Status, message)
}

func answerMessage(answerBody []byte) string {
	var answer struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Detail struct {
			Message string `json:"message"`
		} `json:"detail"`
	}
	if json.Unmarshal(answerBody, &answer) != nil {
		return ""
	}
	message := strings.TrimSpace(answer.Error.Message + answer.Detail.Message)
	if message == "" {
		return ""
	}
	if characters := []rune(message); len(characters) > longestAnswerMessage {
		message = string(characters[:longestAnswerMessage]) + "…"
	}
	return ": " + message
}
