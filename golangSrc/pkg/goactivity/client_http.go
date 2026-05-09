package goactivity

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var httpClient = &http.Client{}

func start(started time.Time) error {
	_, err := executeCall("http://localhost:8080/start?timestamp="+strconv.FormatInt(started.Unix(), 10), false)
	return err
}

func stop() (string, error) {
	return executeCall("http://localhost:8080/stop", true)
}

func isStopped() (bool, error) {
	s, err := executeCall("http://localhost:8080/isstopped", true)
	if err != nil {
		return false, err
	}

	if s != "true" {
		return false, nil
	}
	return true, nil
}

func elapsed() (string, error) {
	return executeCall("http://localhost:8080/elapsed", true)
}

func executeCall(url string, readResponseBody bool) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	if !readResponseBody {
		return "", nil
	}

	bts, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	dest := strings.TrimSpace(string(bts))
	return dest, nil
}
