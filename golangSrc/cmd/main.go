//go:generate go get golang.org/x/mobile/bind
//go:generate go install golang.org/x/mobile/cmd/gomobile@latest
//go:generate go install golang.org/x/mobile/cmd/gobind@latest
//go:generate gomobile init

//go:generate fyne package -name "Fyne Foreground" -os android -tags "android/arm64"

//go:generate unzip -o "Fyne_Foreground.apk" -d ./unzippedAPK
//https://github.com/pxb1988/dex2jar/releases
//go:generate C:/dex-tools-v2.4/d2j-dex2jar ./unzippedAPK/classes.dex -o ./activity.jar --force

//go:generate go run ../utils/filemover.go ../../androidAPK/app/src/main/libs activity.jar
//go:generate go run ../utils/filemover.go ../../androidAPK/app/src/main/jniLibs unzippedAPK/lib/*

package main

import (
	_ "embed"
	"log"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"github.com/leohalb/fyneforeground/pkg/goactivity"
	"github.com/leohalb/fyneforeground/pkg/goservice"
)

//go:embed Icon.png
var resourceIconPngData []byte
var resourceIconPng = &fyne.StaticResource{
	StaticName:    "Icon.png",
	StaticContent: resourceIconPngData,
}

func main() {
	a := app.NewWithID("com.leohalb.fyneforeground")
	a.SetIcon(resourceIconPng)

	if runtime.GOOS != "android" {
		go func() {
			err := goservice.StartForegroundService()
			if err != nil {
				log.Fatal(err)
			}
		}()
	}

	w := a.NewWindow("Clock")
	split := container.NewCenter(goactivity.Show())
	w.SetContent(split)
	w.Resize(fyne.NewSize(480, 360))
	w.ShowAndRun()
}
