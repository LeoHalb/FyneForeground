//go:generate gomobile bind -target=android/arm64 -androidapi 26 -o goservice.aar -ldflags=-checklinkname=0 -v github.com/leohalb/fyneforeground/pkg/goservice

//go:generate go run ../../utils/filemover.go ../../../androidAPK/app/src/main/libs goservice.aar

package goservice

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

var timestamp time.Time
var stoppedTime time.Duration

var webserver *http.Server

func StartForegroundService() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/start", func(_ http.ResponseWriter, r *http.Request) {
		s := r.URL.Query().Get("timestamp")

		unix, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return
		}

		timestamp = time.Unix(unix, 0)
	})

	mux.HandleFunc("/stop", func(w http.ResponseWriter, _ *http.Request) {
		if timestamp.IsZero() {
			stoppedTime = 0
			_, _ = io.WriteString(w, "0s")
			return
		}

		stoppedTime = time.Since(timestamp)
		timestamp = time.Time{}

		_, _ = io.WriteString(w, stoppedTime.Truncate(time.Second).String())
	})

	mux.HandleFunc("/isstopped", func(w http.ResponseWriter, _ *http.Request) {
		if stoppedTime != 0 {
			_, _ = io.WriteString(w, "true")
			return
		}

		_, _ = io.WriteString(w, "false")
	})

	mux.HandleFunc("/elapsed", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, getElapsed())
	})

	webserver = &http.Server{
		Addr:    "localhost:8080",
		Handler: mux,
	}

	// Blocks until server stops.
	err := webserver.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil // expected on shutdown
	}
	return err
}

func StopForegroundService() error {
	if webserver == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	return webserver.Shutdown(ctx)
}

func GetNotificationContent() string {
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
