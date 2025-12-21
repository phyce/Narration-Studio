package stats

import (
	"nstudio/app/common/daemon"
	"nstudio/app/config"
	"os"
	"sync/atomic"
	"time"
)

var (
	processedMessages int64
	startTime         time.Time
)

func Initialize() {
	startTime = time.Now()
	atomic.StoreInt64(&processedMessages, 0)

	updateDaemonStatusFile()

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			updateDaemonStatusFile()
		}
	}()
}

func IncrementMessages() {
	atomic.AddInt64(&processedMessages, 1)
}

func updateDaemonStatusFile() {
	status := daemon.DaemonStatusInfo{
		PID:               os.Getpid(),
		Version:           config.GetInfo().Version,
		StartTime:         startTime,
		ProcessedMessages: atomic.LoadInt64(&processedMessages),
	}

	go daemon.WriteDaemonStatus(status)
}

func GetProcessedMessages() int64 {
	return atomic.LoadInt64(&processedMessages)
}

func GetUptime() time.Duration {
	if startTime.IsZero() {
		return 0
	}
	return time.Since(startTime)
}

func GetStartTime() time.Time {
	return startTime
}
