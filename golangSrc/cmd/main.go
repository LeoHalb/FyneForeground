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

//go:generate go generate ../pkg/goservice/goservice_http.go

////go:generate .\\..\\..\\androidAPK\\gradlew.bat -p ..\..\androidAPK assembleDebug
////go:generate adb install ../../androidAPK/app/build/outputs/apk/debug/app-debug.apk

package main

import (
	_ "embed"
	"log"
	"runtime"

	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver"
	"github.com/AndroidGoLab/jni"
	jniApp "github.com/AndroidGoLab/jni/app"
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
	} else {
		startGoForegroundService()
	}

	w := a.NewWindow("Clock")
	split := container.NewCenter(goactivity.Show())
	w.SetContent(split)
	w.Resize(fyne.NewSize(480, 360))
	w.ShowAndRun()
}

func startGoForegroundService() {
	driver.RunNative(func(ctx interface{}) error {
		ac := ctx.(*driver.AndroidContext)
		env := jni.EnvFromUintptr(ac.Env)
		activity := jni.ObjectFromUintptr(ac.Ctx)

		if err := jniApp.Init(env); err != nil {
			return fmt.Errorf("app.Init: %w", err)
		}

		// Find GoForegroundService class via class loader
		actCls := env.GetObjectClass(activity)
		getClMid, err := env.GetMethodID(actCls, "getClassLoader", "()Ljava/lang/ClassLoader;")
		if err != nil {
			return err
		}
		classLoader, err := env.CallObjectMethod(activity, getClMid)
		if err != nil {
			return err
		}
		clCls := env.GetObjectClass(classLoader)
		loadMid, err := env.GetMethodID(clCls, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;")
		if err != nil {
			return err
		}
		clsName, err := env.NewStringUTF("com.leohalb.fyneforeground.GoForegroundService")
		if err != nil {
			return err
		}
		serviceClass, err := env.CallObjectMethod(classLoader, loadMid, jni.ObjectValue(&clsName.Object))
		if err != nil {
			return fmt.Errorf("loadClass: %w", err)
		}

		// Create Intent(Context, Class)
		intentCls, err := env.FindClass("android/content/Intent")
		if err != nil {
			return err
		}
		intentInit, err := env.GetMethodID(intentCls, "<init>", "(Landroid/content/Context;Ljava/lang/Class;)V")
		if err != nil {
			return err
		}
		intent, err := env.NewObject(intentCls, intentInit,
			jni.ObjectValue(activity), jni.ObjectValue(serviceClass))
		if err != nil {
			return fmt.Errorf("new Intent: %w", err)
		}

		// Call startForegroundService(intent)
		ctxCls, err := env.FindClass("android/content/Context")
		if err != nil {
			return err
		}
		startMid, err := env.GetMethodID(ctxCls, "startForegroundService", "(Landroid/content/Intent;)Landroid/content/ComponentName;")
		if err != nil {
			return err
		}
		_, err = env.CallObjectMethod(activity, startMid, jni.ObjectValue(intent))
		if err != nil {
			return fmt.Errorf("startForegroundService: %w", err)
		}

		log.Println("GoForegroundService started from Go")
		return nil
	})
}
