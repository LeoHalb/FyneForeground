package goactivity

import (
	"fmt"
	"log"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/widget"
	"github.com/AndroidGoLab/jni"
	"github.com/AndroidGoLab/jni/app"
	"github.com/AndroidGoLab/jni/content"
	"runtime"
	"time"
)

const (
	actionElapsed    = "com.leohalb.fyneforeground.ELAPSED_UPDATE"
	actionStartTimer = "com.leohalb.fyneforeground.START_TIMER"
	actionStopTimer  = "com.leohalb.fyneforeground.STOP_TIMER"
	extraElapsed     = "elapsed"
	extraTimestamp   = "timestamp"
)

// elapsedCh receives elapsed time strings from the BroadcastReceiver
var elapsedCh = make(chan string, 1)

func Show() fyne.CanvasObject {
	isRunning := false
	label := widget.NewLabel("0s")

	if runtime.GOOS == "android" {
		// Register BroadcastReceiver to get elapsed updates from the service
		initBroadcastReceiver()

		// Listen for broadcasts and update label
		go func() {
			for elapsed := range elapsedCh {
				e := elapsed
				fyne.Do(func() {
					label.SetText(e)
				})
			}
		}()
	}

	// Desktop: poll via HTTP as before
	if runtime.GOOS != "android" {
		ticker := time.NewTicker(time.Second)
		go func() {
			for {
				if isRunning {
					elapsedTime, err := elapsed()
					if err != nil {
						log.Println("error getting elapsed time: ", err)
					}
					fyne.Do(func() {
						label.SetText(elapsedTime)
					})
				}
				<-ticker.C
			}
		}()
	}

	button := widget.NewButton("Start", nil)
	button.OnTapped = func() {
		if !isRunning {
			if runtime.GOOS == "android" {
				requestPostNotificationPermission()
				sendCommandToService(actionStartTimer, extraTimestamp, int32(time.Now().Unix()))
			} else {
				start(time.Now())
			}
			fyne.Do(func() {
				label.SetText("0s")
				button.Text = "Stop"
				button.Refresh()
			})
		} else {
			if runtime.GOOS == "android" {
				sendCommandToService(actionStopTimer, "", 0)
			} else {
				stop()
			}
			fyne.Do(func() {
				button.Text = "Start"
				button.Refresh()
			})
		}
		isRunning = !isRunning
	}

	return container.NewVBox(label, button)
}

func initBroadcastReceiver() {
	log.Println("initBroadcastReceiver started")
	//var cleanup func()

	err := driver.RunNative(func(ctx interface{}) error {
		ac := ctx.(*driver.AndroidContext)
		vm := jni.VMFromUintptr(ac.VM)
		env := jni.EnvFromUintptr(ac.Env)
		activity := jni.ObjectFromUintptr(ac.Ctx)

		// Make a global ref so it survives beyond RunNative
		activityGlobal := env.NewGlobalRef(activity)

		// Set the ClassLoader so proxy init can find GoInvocationHandler
		actCls := env.GetObjectClass(activity)
		getClMid, _ := env.GetMethodID(actCls, "getClassLoader", "()Ljava/lang/ClassLoader;")
		classLoader, _ := env.CallObjectMethod(activity, getClMid)
		clGlobal := env.NewGlobalRef(classLoader)
		jni.SetProxyClassLoader(clGlobal)

		// 1. Register the Go handler that will receive onReceive calls.
		handlerID := jni.RegisterProxyHandler(
			func(env *jni.Env, methodName string, args []*jni.Object) (*jni.Object, error) {
				if methodName != "onReceive" {
					return nil, nil
				}
				// args[0] = Context, args[1] = Intent

				// Use the typed Intent wrapper to read action and extras.
				intent := &app.Intent{VM: vm, Obj: args[1]}
				elapsed, err := intent.GetStringExtra(extraElapsed)
				if err != nil {
					return nil, err
				}
				// Non-blocking send
				select {
				case elapsedCh <- elapsed:
				default:
				}
				return nil, nil
			},
		)

		// 2. Create an IntentFilter for the desired action.
		filter, err := content.NewIntentFilter(vm)
		if err != nil {
			jni.UnregisterProxyHandler(handlerID)
			return fmt.Errorf("new IntentFilter: %w", err)
		}
		err = filter.AddAction(actionElapsed)
		if err != nil {
			jni.UnregisterProxyHandler(handlerID)
			return fmt.Errorf("filter.AddAction: %w", err)
		}

		// 3. Instantiate GoBroadcastReceiver (custom Java adapter -- raw JNI required).
		var receiverGlobal *jni.Object
		if err := jni.EnsureProxyInit(env); err != nil {
			return fmt.Errorf("EnsureProxyInit: %w", err)
		}

		// GoBroadcastReceiver is a user-defined Java class. In NativeActivity,
		// FindClass uses the boot ClassLoader which cannot see APK classes.
		// Use the Activity's ClassLoader instead.
		getClassLoaderMid, err := env.GetMethodID(actCls, "getClassLoader",
			"()Ljava/lang/ClassLoader;")
		if err != nil {
			return fmt.Errorf("get getClassLoader: %w", err)
		}
		classLoader, err = env.CallObjectMethod(activityGlobal, getClassLoaderMid)
		if err != nil {
			return fmt.Errorf("getClassLoader: %w", err)
		}

		clCls := env.GetObjectClass(classLoader)
		loadClassMid, err := env.GetMethodID(clCls, "loadClass",
			"(Ljava/lang/String;)Ljava/lang/Class;")
		if err != nil {
			return fmt.Errorf("get loadClass: %w", err)
		}

		jName, err := env.NewStringUTF("com.leohalb.fyneforeground.GoBroadcastReceiver")
		if err != nil {
			return err
		}
		defer env.DeleteLocalRef(&jName.Object)
		receiverCls, err := env.CallObjectMethod(classLoader, loadClassMid,
			jni.ObjectValue(&jName.Object))
		if err != nil {
			return fmt.Errorf("loadClass(GoBroadcastReceiver): %w", err)
		}

		cls := (*jni.Class)(unsafe.Pointer(receiverCls))
		initMid, err := env.GetMethodID(cls, "<init>", "(J)V")
		if err != nil {
			return fmt.Errorf("get GoBroadcastReceiver.<init>: %w", err)
		}
		receiver, err := env.NewObject(cls, initMid, jni.LongValue(handlerID))
		if err != nil {
			return fmt.Errorf("new GoBroadcastReceiver: %w", err)
		}
		receiverGlobal = env.NewGlobalRef(receiver)

		// 4. Register the receiver with the Context.
		const RECEIVER_NOT_EXPORTED = 4 // Context.RECEIVER_NOT_EXPORTED
		appCtx := &app.Context{VM: vm, Obj: activityGlobal}
		_, err = appCtx.RegisterReceiver3_1(receiverGlobal, filter.Obj, RECEIVER_NOT_EXPORTED)
		if err != nil {
			filter.Close()
			jni.UnregisterProxyHandler(handlerID)
			return fmt.Errorf("registerReceiver: %w", err)
		}

		// 5. Return a cleanup function.
		_ = func() {
			ctx := &app.Context{VM: vm, Obj: activityGlobal}
			ctx.UnregisterReceiver(receiverGlobal)
			vm.Do(func(env *jni.Env) error {
				env.DeleteGlobalRef(receiverGlobal)
				return nil
			})
			filter.Close()
			jni.UnregisterProxyHandler(handlerID)
		}

		log.Println("BroadcastReceiver registered for elapsed updates")
		return nil
	})
	if err != nil {
		log.Println("initBroadcastReceiver err:", err)
	}
}

func sendCommandToService(action string, extraKey string, extraValue int32) {
	err := driver.RunNative(func(ctx interface{}) error {
		ac := ctx.(*driver.AndroidContext)
		vm := jni.VMFromUintptr(ac.VM)
		activity := jni.ObjectFromUintptr(ac.Ctx)

		return vm.Do(func(env *jni.Env) error {
			// Create Intent() using no-arg constructor
			intentCls, err := env.FindClass("android/content/Intent")
			if err != nil {
				return fmt.Errorf("find Intent class: %w", err)
			}
			initMid, err := env.GetMethodID(intentCls, "<init>", "()V")
			if err != nil {
				return fmt.Errorf("get Intent.<init>: %w", err)
			}
			intent, err := env.NewObject(intentCls, initMid)
			if err != nil {
				return fmt.Errorf("new Intent: %w", err)
			}

			// setAction
			setActionMid, err := env.GetMethodID(intentCls, "setAction",
				"(Ljava/lang/String;)Landroid/content/Intent;")
			if err != nil {
				return err
			}
			jAction, _ := env.NewStringUTF(action)
			env.CallObjectMethod(intent, setActionMid, jni.ObjectValue(&jAction.Object))

			// setClassName
			setClassMid, _ := env.GetMethodID(intentCls, "setClassName",
				"(Ljava/lang/String;Ljava/lang/String;)Landroid/content/Intent;")
			jPkg, _ := env.NewStringUTF("com.leohalb.fyneforeground")
			jCls, _ := env.NewStringUTF("com.leohalb.fyneforeground.GoForegroundService")
			env.CallObjectMethod(intent, setClassMid,
				jni.ObjectValue(&jPkg.Object), jni.ObjectValue(&jCls.Object))

			// putExtra if needed
			if extraKey != "" {
				putIntMid, _ := env.GetMethodID(intentCls, "putExtra",
					"(Ljava/lang/String;I)Landroid/content/Intent;")
				jKey, _ := env.NewStringUTF(extraKey)
				env.CallObjectMethod(intent, putIntMid,
					jni.ObjectValue(&jKey.Object), jni.IntValue(extraValue))
			}

			// startService
			ctxCls, _ := env.FindClass("android/content/Context")
			startMid, _ := env.GetMethodID(ctxCls, "startService",
				"(Landroid/content/Intent;)Landroid/content/ComponentName;")
			_, err = env.CallObjectMethod(activity, startMid, jni.ObjectValue(intent))
			if err != nil {
				return fmt.Errorf("startService: %w", err)
			}

			log.Printf("sent intent: action=%s", action)
			return nil
		})
	})
	if err != nil {
		log.Println("sendCommandToService error:", err)
	}
}

func requestPostNotificationPermission() error {
	return driver.RunNative(func(ctx interface{}) error {
		ac := ctx.(*driver.AndroidContext)
		jniVM := jni.VMFromUintptr(ac.VM)

		// Wrap Fyne's context (the Activity) as a jni.Object
		activity := jni.ObjectFromUintptr(ac.Ctx)

		return jniVM.Do(func(env *jni.Env) error {
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
	})
}
