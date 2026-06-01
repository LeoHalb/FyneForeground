package goactivity

import (
	"fmt"
	"log"
	"sync"
	"time"
	"unsafe"

	"fyne.io/fyne/v2/driver"
	"github.com/AndroidGoLab/jni"
)

// binderRef holds a global ref to the IGoService AIDL proxy (cross-process binder)
var (
	binderRef *jni.Object
	binderVM  *jni.VM
	binderMu  sync.Mutex
	boundCh   = make(chan struct{}, 1)
)

// bindToService binds to GoForegroundService via AIDL and stores the binder proxy.
// Must be called from inside driver.RunNative or on a thread with a valid JNI env.
func bindToService() error {
	return driver.RunNative(func(ctx interface{}) error {
		ac := ctx.(*driver.AndroidContext)
		vm := jni.VMFromUintptr(ac.VM)
		env := jni.EnvFromUintptr(ac.Env)
		activity := jni.ObjectFromUintptr(ac.Ctx)

		binderVM = vm

		// Set up proxy ClassLoader for EnsureProxyInit
		actCls := env.GetObjectClass(activity)
		getClMid, _ := env.GetMethodID(actCls, "getClassLoader", "()Ljava/lang/ClassLoader;")
		classLoader, _ := env.CallObjectMethod(activity, getClMid)
		clGlobal := env.NewGlobalRef(classLoader)
		jni.SetProxyClassLoader(clGlobal)

		if err := jni.EnsureProxyInit(env); err != nil {
			return fmt.Errorf("EnsureProxyInit: %w", err)
		}

		// Find ServiceConnection interface
		scCls, err := env.FindClass("android/content/ServiceConnection")
		if err != nil {
			return fmt.Errorf("find ServiceConnection: %w", err)
		}

		// Create proxy for ServiceConnection
		proxy, _, err := env.NewProxy(
			[]*jni.Class{(*jni.Class)(unsafe.Pointer(scCls))},
			func(env *jni.Env, methodName string, args []*jni.Object) (*jni.Object, error) {
				switch methodName {
				case "onServiceConnected":
					// args[0] = ComponentName, args[1] = IBinder
					ibinder := args[1]

					// Call IGoService.Stub.asInterface(ibinder) to get the AIDL proxy
					stubCls, err := env.FindClass("com/leohalb/fyneforeground/IGoService$Stub")
					if err != nil {
						log.Println("IGoService.Stub not found:", err)
						return nil, nil
					}
					asIfaceMid, err := env.GetStaticMethodID(
						stubCls, "asInterface",
						"(Landroid/os/IBinder;)Lcom/leohalb/fyneforeground/IGoService;")
					if err != nil {
						log.Println("asInterface not found:", err)
						return nil, nil
					}
					aidlProxy, err := env.CallStaticObjectMethod(stubCls, asIfaceMid,
						jni.ObjectValue(ibinder))
					if err != nil {
						log.Println("asInterface failed:", err)
						return nil, nil
					}

					binderMu.Lock()
					binderRef = env.NewGlobalRef(aidlProxy)
					binderMu.Unlock()

					log.Println("AIDL service bound successfully")

					// Signal that binding is ready
					select {
					case boundCh <- struct{}{}:
					default:
					}

				case "onServiceDisconnected":
					binderMu.Lock()
					if binderRef != nil {
						vm.Do(func(env *jni.Env) error {
							env.DeleteGlobalRef(binderRef)
							return nil
						})
						binderRef = nil
					}
					binderMu.Unlock()
					log.Println("AIDL service disconnected")
				}
				return nil, nil
			},
		)
		if err != nil {
			return fmt.Errorf("NewProxy ServiceConnection: %w", err)
		}

		// Store proxy as global ref so it survives
		proxyGlobal := env.NewGlobalRef(proxy)

		// Create Intent targeting GoForegroundService
		intentCls, _ := env.FindClass("android/content/Intent")
		initMid, _ := env.GetMethodID(intentCls, "<init>", "()V")
		intent, _ := env.NewObject(intentCls, initMid)

		setClassMid, _ := env.GetMethodID(intentCls, "setClassName",
			"(Ljava/lang/String;Ljava/lang/String;)Landroid/content/Intent;")
		jPkg, _ := env.NewStringUTF("com.leohalb.fyneforeground")
		jCls, _ := env.NewStringUTF("com.leohalb.fyneforeground.GoForegroundService")
		env.CallObjectMethod(intent, setClassMid,
			jni.ObjectValue(&jPkg.Object), jni.ObjectValue(&jCls.Object))

		// bindService(intent, conn, BIND_AUTO_CREATE)
		ctxCls, _ := env.FindClass("android/content/Context")
		bindMid, _ := env.GetMethodID(ctxCls, "bindService",
			"(Landroid/content/Intent;Landroid/content/ServiceConnection;I)Z")

		const BIND_AUTO_CREATE = 1
		_, _ = env.CallBooleanMethod(activity, bindMid,
			jni.ObjectValue(intent),
			jni.ObjectValue(proxyGlobal),
			jni.IntValue(BIND_AUTO_CREATE))

		log.Println("bindService called, waiting for onServiceConnected...")
		return nil
	})
}

// callStringMethod calls a no-arg method that returns String on the AIDL binder proxy
func callStringMethod(methodName string) (string, error) {
	binderMu.Lock()
	ref := binderRef
	vm := binderVM
	binderMu.Unlock()

	if ref == nil || vm == nil {
		return "", fmt.Errorf("service not bound")
	}

	var result string
	err := vm.Do(func(env *jni.Env) error {
		cls := env.GetObjectClass(ref)
		mid, err := env.GetMethodID(cls, methodName, "()Ljava/lang/String;")
		if err != nil {
			return fmt.Errorf("%s not found: %w", methodName, err)
		}
		obj, err := env.CallObjectMethod(ref, mid)
		if err != nil {
			return err
		}
		if obj != nil {
			result = env.GoString((*jni.String)(unsafe.Pointer(obj)))
		}
		return nil
	})
	return result, err
}

// aidlStart calls IGoService.start(timestamp) via AIDL binder
func aidlStart(started time.Time) error {
	binderMu.Lock()
	ref := binderRef
	vm := binderVM
	binderMu.Unlock()

	if ref == nil || vm == nil {
		return fmt.Errorf("service not bound")
	}

	return vm.Do(func(env *jni.Env) error {
		cls := env.GetObjectClass(ref)
		mid, err := env.GetMethodID(cls, "start", "(J)V")
		if err != nil {
			return fmt.Errorf("start not found: %w", err)
		}
		return env.CallVoidMethod(ref, mid, jni.LongValue(started.Unix()))
	})
}

// aidlStop calls IGoService.stop() via AIDL binder
func aidlStop() (string, error) {
	return callStringMethod("stop")
}

// aidlElapsed calls IGoService.getElapsed() via AIDL binder
func aidlElapsed() (string, error) {
	return callStringMethod("getElapsed")
}

// aidlIsRunning calls IGoService.isRunning() via AIDL binder
func aidlIsRunning() (bool, error) {
	binderMu.Lock()
	ref := binderRef
	vm := binderVM
	binderMu.Unlock()

	if ref == nil || vm == nil {
		return false, fmt.Errorf("service not bound")
	}

	var result bool
	err := vm.Do(func(env *jni.Env) error {
		cls := env.GetObjectClass(ref)
		mid, err := env.GetMethodID(cls, "isRunning", "()Z")
		if err != nil {
			return fmt.Errorf("isRunning not found: %w", err)
		}
		val, err := env.CallBooleanMethod(ref, mid)
		if err != nil {
			return err
		}
		result = val != 0
		return nil
	})
	return result, err
}
