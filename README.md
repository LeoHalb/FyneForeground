## Fyne Foreground

A proof of concept for running a [Fyne](https://fyne.io/) UI application alongside an Android foreground service, written in Go.

Android foreground services are the way to go for implementing long-running background tasks on Android, but they require a persistent notification and a Java/Kotlin implementation. This project shows how to wrap a Go HTTP server (`goservice.go`) inside a foreground service (`GoForegroundService.java`) and have a Fyne UI (`goactivity.go`) communicate with it over HTTP, allowing the timer to keep running even when the app is backgrounded.

⚠️ For now, there's no prompt to allow the app to send notifications, leaving you with a higher chance that the Android OS will kill the app. You have to manually enable the permission in the settings. ⚠️

## Overview

The app displays a simple stopwatch. On desktop, it runs entirely in Go. On Android, the timer state is kept alive by an Android foreground service (`GoForegroundService`) so the clock keeps ticking even when the app is backgrounded.

### Architecture

```
┌─────────────────────────────────────┐
│           Fyne UI (goactivity)      │  ← Go, compiled via fyne package
│  Polls goservice over HTTP          │
└────────────────┬────────────────────┘
                 │ localhost:8080
┌────────────────▼────────────────────┐
│        goservice (HTTP server)      │  ← Go, compiled via gomobile bind → goservice.aar
│  Holds timestamp / stoppedTime      │
│  Endpoints: /start /stop /elapsed   │
│             /isstopped              │
└─────────────────────────────────────┘
         hosted inside
┌─────────────────────────────────────┐
│      GoForegroundService (Java)     │  ← Android foreground service
│  Keeps process alive in background  │
│  Updates persistent notification    │
└─────────────────────────────────────┘
```

On **desktop**, `goservice` is started in-process as a goroutine.  
On **Android**, `GoForegroundService` starts `goservice` in its own thread and the Fyne activity communicates with it over HTTP on `localhost:8080`.

## Prerequisites

| Tool | Version |
|------|---------|
| Go | 1.21+ |
| Android SDK | API 26+ |
| Android NDK | r25+ |
| gomobile | latest |
| fyne CLI | latest |
| dex2jar | [v2.4](https://github.com/pxb1988/dex2jar/releases) |

## Building

The build is split into two steps: building the Go side and then the Android APK.

### 1. Build the Go side

From `golangSrc/cmd/`:

```bash
go generate
```

This runs the following steps in order:

1. Installs `gomobile` and `gobind`
2. Runs `fyne package` to produce `Fyne_Foreground.apk` (the activity `.so` + `classes.dex`)
3. Unzips the APK and converts `classes.dex` → `activity.jar` via dex2jar
4. Copies `activity.jar` and the JNI `.so` libs into the Android project (`androidAPK/app/src/main/libs` and `jniLibs`)
5. Runs `gomobile bind` on `goservice` to produce `goservice.aar` and copies it into the Android project

### 2. Build the Android APK

```bash
cd androidAPK
./gradlew assembleDebug
```

The output APK is at `androidAPK/app/build/outputs/apk/debug/app-debug.apk`.

### Install on device

```bash
adb install androidAPK/app/build/outputs/apk/debug/app-debug.apk
```

## Running on Desktop

```bash
cd golangSrc
go run ./cmd
```

## Project Structure

```
FyneForeground/
├── golangSrc/
│   ├── cmd/
│   │   └── main.go          # Entry point; go:generate directives
│   ├── pkg/
│   │   ├── goactivity/
│   │   │   ├── goactivity.go # Fyne UI, talks to goservice over HTTP
│   │   │   ├── client_http.go# HTTP client helpers
│   │   └── goservice/
│   │       └── goservice_http.go # HTTP server holding timer state
│   └── utils/
│       └── filemover.go     # Build helper: copies files into the Android project
└── androidAPK/
    └── app/src/main/java/com/leohalb/fyneforeground/
        ├── MainActivity.java       # Hosts the Fyne activity
        └── GoForegroundService.java# Android foreground service wrapping goservice
```

## How It Works

1. **Fyne UI** (`goactivity`) renders the stopwatch label and Start/Stop button.
2. On Start, it calls `goservice` (via HTTP) to record the current timestamp.
3. `goservice` keeps the timestamp in memory and serves elapsed time on `/elapsed`.
4. On Android, `GoForegroundService` hosts `goservice` in its own thread, keeping the process alive in the background and updating the notification every second.
5. The Fyne UI polls `/elapsed` every second to update the label, whether in the foreground or after resuming from background.

## Notes

- The `goservice.aar` and the JNI libs extracted from the Fyne APK are committed separately into the Android project after each Go build (`go generate`).
- dex2jar is required because the Fyne activity classes are needed as a compile-time dependency for the Android project.
- On desktop, `goservice` runs as an in-process goroutine on `localhost:8080` — no Android SDK required.
