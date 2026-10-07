package pack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"nstudio/app/common/response"
)

var (
	downloadStallTimeout           = 60 * time.Second
	attemptsWithoutProgressAllowed = 5
	firstRetryDelay                = 2 * time.Second
	longestRetryDelay              = 30 * time.Second
	errDownloadStalled             = errors.New("no data received")
	errDownloadCorrupted           = errors.New("the downloaded file is corrupted")
	errNotWorthRetrying            = errors.New("not worth retrying")
	downloadClient                 = newDownloadClient()
)

func newDownloadClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 60 * time.Second
	transport.DisableCompression = true
	return &http.Client{Transport: transport}
}

func downloadResumable(ctx context.Context, url, destinationPath string, expectedSize int64, expectedSHA256 string, progress *installProgress) error {
	if _, err := os.Stat(destinationPath); err == nil {
		if verifyDownload(destinationPath, expectedSize, expectedSHA256) == nil {
			progress.advanceTo(expectedSize, expectedSize)
			return nil
		}
		_ = os.Remove(destinationPath)
	}

	partialPath := destinationPath + ".part"
	err := retryWhileProgressing(ctx, func() (bool, error) {
		return downloadAttempt(ctx, url, partialPath, expectedSize, progress)
	})
	if err != nil {
		return err
	}

	if err := verifyDownload(partialPath, expectedSize, expectedSHA256); err != nil {
		_ = os.Remove(partialPath)
		return err
	}
	if err := os.Rename(partialPath, destinationPath); err != nil {
		return response.Err("%w", err)
	}
	return nil
}

func retryWhileProgressing(ctx context.Context, attempt func() (madeProgress bool, err error)) error {
	attemptsWithoutProgress := 0
	retryDelay := firstRetryDelay
	for {
		madeProgress, err := attempt()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, errNotWorthRetrying) {
			return err
		}
		if madeProgress {
			attemptsWithoutProgress = 0
			retryDelay = firstRetryDelay
		} else {
			attemptsWithoutProgress++
		}
		if attemptsWithoutProgress >= attemptsWithoutProgressAllowed {
			return response.Err("gave up after %d attempts without progress: %w", attemptsWithoutProgress, err)
		}
		response.NewWarn(fmt.Sprintf("Download interrupted (%v); retrying in %s", err, retryDelay))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryDelay):
		}
		retryDelay = min(retryDelay*2, longestRetryDelay)
	}
}

func downloadAttempt(ctx context.Context, url, partialPath string, expectedSize int64, progress *installProgress) (bool, error) {
	var resumeOffset int64
	if partialInfo, err := os.Stat(partialPath); err == nil {
		resumeOffset = partialInfo.Size()
	}
	if expectedSize > 0 && resumeOffset == expectedSize {
		progress.advanceTo(resumeOffset, expectedSize)
		return false, nil
	}
	if expectedSize > 0 && resumeOffset > expectedSize {
		_ = os.Remove(partialPath)
		resumeOffset = 0
	}

	attemptContext, cancelAttempt := context.WithCancel(ctx)
	defer cancelAttempt()

	request, err := http.NewRequestWithContext(attemptContext, http.MethodGet, url, nil)
	if err != nil {
		return false, response.Err("%w", err)
	}
	request.Header.Set("User-Agent", "NarrationStudio")
	if resumeOffset > 0 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeOffset))
	}

	downloadResponse, err := downloadClient.Do(request)
	if err != nil {
		return false, err
	}
	defer downloadResponse.Body.Close()

	openFlags := os.O_CREATE | os.O_WRONLY
	switch downloadResponse.StatusCode {
	case http.StatusPartialContent:
		openFlags |= os.O_APPEND
	case http.StatusOK:
		resumeOffset = 0
		openFlags |= os.O_TRUNC
	case http.StatusRequestedRangeNotSatisfiable:
		return false, nil
	case http.StatusRequestTimeout, http.StatusTooManyRequests:
		return false, fmt.Errorf("server returned %s", downloadResponse.Status)
	default:
		if downloadResponse.StatusCode >= 400 && downloadResponse.StatusCode < 500 {
			return false, response.Err("server returned %s: %w", downloadResponse.Status, errNotWorthRetrying)
		}
		return false, fmt.Errorf("server returned %s", downloadResponse.Status)
	}

	totalBytes := expectedSize
	if totalBytes <= 0 && downloadResponse.ContentLength > 0 {
		totalBytes = resumeOffset + downloadResponse.ContentLength
	}
	progress.advanceTo(resumeOffset, totalBytes)

	partialFile, err := os.OpenFile(partialPath, openFlags, 0o644)
	if err != nil {
		return false, response.Err("%w: %w", err, errNotWorthRetrying)
	}
	defer partialFile.Close()

	stallTimer := time.AfterFunc(downloadStallTimeout, cancelAttempt)
	defer stallTimer.Stop()

	receivedBytes := int64(0)
	buffer := make([]byte, 256*1024)
	for {
		readCount, readErr := downloadResponse.Body.Read(buffer)
		if readCount > 0 {
			stallTimer.Reset(downloadStallTimeout)
			if _, err := partialFile.Write(buffer[:readCount]); err != nil {
				return receivedBytes > 0, response.Err("write %s: %w: %w", partialPath, err, errNotWorthRetrying)
			}
			receivedBytes += int64(readCount)
			progress.advanceTo(resumeOffset+receivedBytes, totalBytes)
		}
		if readErr == io.EOF {
			return receivedBytes > 0, nil
		}
		if readErr != nil {
			if ctx.Err() == nil && attemptContext.Err() != nil {
				readErr = fmt.Errorf("%w for %s", errDownloadStalled, downloadStallTimeout)
			}
			return receivedBytes > 0, readErr
		}
	}
}

func verifyDownload(filePath string, expectedSize int64, expectedSHA256 string) error {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return response.Err("%w", err)
	}
	if expectedSize > 0 && fileInfo.Size() != expectedSize {
		return response.Err("%w: expected %d bytes, got %d", errDownloadCorrupted, expectedSize, fileInfo.Size())
	}
	if expectedSHA256 == "" {
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return response.Err("%w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return response.Err("%w", err)
	}
	if actualSHA256 := hex.EncodeToString(hash.Sum(nil)); !strings.EqualFold(actualSHA256, expectedSHA256) {
		return response.Err("%w: SHA-256 is %s, expected %s", errDownloadCorrupted, actualSHA256, expectedSHA256)
	}
	return nil
}
