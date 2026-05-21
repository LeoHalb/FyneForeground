//go:generate gomobile bind -target=android/arm64 -androidapi 26 -o goservice.aar -ldflags=-checklinkname=0 -v github.com/leohalb/fyneforeground/pkg/goservice

//go:generate go run ../../utils/filemover.go ../../../androidAPK/app/src/main/libs goservice.aar

package goservice

import (
	"sync"
	"time"
)

var (
	mu          sync.Mutex
	timestamp   time.Time
	stoppedTime time.Duration
	done        chan struct{}
)

func StartForegroundService() error {
	done = make(chan struct{})
	<-done // block until stopped
	return nil
}

func StopForegroundService() error {
	if done != nil {
		close(done)
	}
	return nil
}

func Start(unixTimestamp int64) {
	mu.Lock()
	defer mu.Unlock()
	timestamp = time.Unix(unixTimestamp, 0)
	stoppedTime = 0
}

func Stop() string {
	mu.Lock()
	defer mu.Unlock()
	if timestamp.IsZero() {
		stoppedTime = 0
		return "0s"
	}
	stoppedTime = time.Since(timestamp)
	timestamp = time.Time{}
	return stoppedTime.Truncate(time.Second).String()
}

func IsRunning() bool {
	mu.Lock()
	defer mu.Unlock()
	return !timestamp.IsZero()
}

func GetNotificationContent() string {
	mu.Lock()
	defer mu.Unlock()
	return getElapsed()
}

func getElapsed() string {
	if timestamp.IsZero() {
		if stoppedTime != 0 {
			return stoppedTime.Truncate(time.Second).String()
		}
		return "0s"
	}
	return time.Since(timestamp).Truncate(time.Second).String()
}
