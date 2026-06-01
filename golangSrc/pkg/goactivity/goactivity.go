package goactivity

import (
	"log"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/widget"
	"github.com/AndroidGoLab/jni"
)

func Show() fyne.CanvasObject {
	label := widget.NewLabel("0s")
	var currentlyRunning bool

	// On Android, bind to the AIDL service asynchronously after a short delay
	// to ensure the Fyne native context is ready
	if runtime.GOOS == "android" {
		go func() {
			// Give the window time to initialize the native context
			time.Sleep(500 * time.Millisecond)

			log.Println("Attempting to bind to service...")
			if err := bindToService(); err != nil {
				log.Println("bindToService error:", err)
				return
			}

			// Wait for onServiceConnected
			select {
			case <-boundCh:
				log.Println("Service binding confirmed")
				r, err := aidlIsRunning()
				if err != nil {
					log.Println("aidlIsRunning error:", err)
				} else {
					currentlyRunning = r
				}
			case <-time.After(10 * time.Second):
				log.Println("timed out waiting for service binding")
			}
		}()
	} else {
		r, err := isRunning()
		if err != nil {
			log.Println("isRunning error:", err)
		}
		currentlyRunning = r
	}

	ticker := time.NewTicker(time.Second)
	go func() {
		for {
			if currentlyRunning {
				var elapsedTime string
				var err error
				if runtime.GOOS == "android" {
					elapsedTime, err = aidlElapsed()
				} else {
					elapsedTime, err = elapsed()
				}
				if err != nil {
					log.Println("error getting elapsed time: ", err)
				} else {
					log.Println("elapsed: ", elapsedTime)
					fyne.Do(func() {
						label.SetText(elapsedTime)
					})
				}
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

		if !currentlyRunning {
			ticker.Reset(time.Second)

			if runtime.GOOS == "android" {
				if err := aidlStart(time.Now()); err != nil {
					log.Println("aidlStart error:", err)
				}
			} else {
				if err := start(time.Now()); err != nil {
					log.Println("error starting clock:", err)
				}
			}

			fyne.Do(func() {
				label.SetText("0s")
				button.Text = "Stop"
				button.Refresh()
			})
		} else {
			ticker.Stop()

			var stoppedTime string
			if runtime.GOOS == "android" {
				s, err := aidlStop()
				if err != nil {
					log.Println("aidlStop error:", err)
				}
				stoppedTime = s
			} else {
				s, err := stop()
				if err != nil {
					log.Println("error stopping clock:", err)
				}
				stoppedTime = s
			}

			fyne.Do(func() {
				label.SetText(stoppedTime)
				button.Text = "Start"
				button.Refresh()
			})
		}

		currentlyRunning = !currentlyRunning
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
