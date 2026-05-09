package goactivity

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"log"
)

func Show() fyne.CanvasObject {
	isStopped, err := isStopped()
	if err != nil {
		log.Fatal("error getting isStopped: ", err)
	}

	label := widget.NewLabel("0s")

	ticker := time.NewTicker(time.Second)
	go func() {
		for {
			if isStopped {
				elapsedTime, err := elapsed()
				if err != nil {
					log.Fatal("error getting elapsed time: ", err)
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
		if !isStopped {
			ticker.Reset(time.Second)

			err := start(time.Now())
			if err != nil {
				log.Print("error starting clock: ", err)
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
				log.Print("error stopping clock: ", err)
			}

			fyne.Do(func() {
				label.SetText(stoppedTime)
				button.Text = "Start"
				button.Refresh()
			})
		}

		isStopped = !isStopped
	}

	c := container.NewVBox(label, button)

	return c
}
