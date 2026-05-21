package goactivity

import (
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/widget"
	"github.com/AndroidGoLab/jni"
	"runtime"
)

func Show() fyne.CanvasObject {
	isRunning, err := isRunning()
	if err != nil {
		log.Println("error getting isRunning: ", err)
	}

	label := widget.NewLabel("0s")

	ticker := time.NewTicker(time.Second)
	go func() {
		for {
			if isRunning {
				elapsedTime, err := elapsed()
				if err != nil {
					log.Println("error getting elapsed time: ", err)
				}

				// you will see this in Logcat
				log.Println("elapsed: ", elapsedTime)

				fyne.Do(func() {
					label.SetText(elapsedTime)
				})
			}
			<-ticker.C
		}
	}()

	button := widget.NewButton("Start", nil)
	button.OnTapped = func() {
		if runtime.GOOS == "android" {
			err := requestPostNotificationPermission()
			if err != nil {
				log.Println("error posting notification: ", err)
			}
		}

		if !isRunning {
			ticker.Reset(time.Second)

			err := start(time.Now())
			if err != nil {
				log.Println("error starting clock: ", err)
			}

			fyne.Do(func() {
				label.SetText("0s")
				button.Text = "Stop"
				button.Refresh()
			})
		} else {
			ticker.Stop()

			stoppedTime, err := stop()
			if err != nil {
				log.Println("error stopping clock: ", err)
			}

			fyne.Do(func() {
				label.SetText(stoppedTime)
				button.Text = "Start"
				button.Refresh()
			})
		}

		isRunning = !isRunning
	}

	c := container.NewVBox(label, button)

	return c
}

func requestPostNotificationPermission() error {
	return driver.RunNative(func(ctx interface{}) error {
		ac := ctx.(*driver.AndroidContext)
		env := jni.EnvFromUintptr(ac.Env)

		// Wrap Fyne's context (the Activity) as a jni.Object
		activity := jni.ObjectFromUintptr(ac.Ctx)

		actCls := env.GetObjectClass(activity)

		reqMid, err := env.GetMethodID(actCls, "requestPermissions", "([Ljava/lang/String;I)V")
		if err != nil {
			log.Println("requestPermissions method not found: ", err)
			return err
		}

		strCls, err := env.FindClass("java/lang/String")
		if err != nil {
			return err
		}

		perms := []string{"android.permission.POST_NOTIFICATIONS"}
		arr, err := env.NewObjectArray(int32(len(perms)), strCls, nil)
		if err != nil {
			return err
		}

		for i, p := range perms {
			jP, err := env.NewStringUTF(p)
			if err != nil {
				return err
			}
			_ = env.SetObjectArrayElement(arr, int32(i), &jP.Object)
		}

		_ = env.CallVoidMethod(activity, reqMid,
			jni.ObjectValue(&arr.Object), jni.IntValue(1))

		return nil
	})
}
