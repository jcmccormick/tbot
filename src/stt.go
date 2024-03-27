package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"time"
)

func errCheck(err error) {
	if err != nil {
		panic(err)
	}
}

func calculateRMS(buffer []byte) float64 {
	var sumSquare float64
	for _, sample := range buffer {
		// Assuming 8-bit samples. Adjust for 16-bit samples if needed.
		normalizedSample := float64(sample) - 128 // Centering 8-bit samples around 0
		sumSquare += normalizedSample * normalizedSample
	}
	meanSquare := sumSquare / float64(len(buffer))
	return math.Sqrt(meanSquare)
}

func GetTextFromSpeech(text chan<- string) {

	var header []byte
	var clip bytes.Buffer
	var recCmd *exec.Cmd
	var recStdout io.ReadCloser

	recordingFile := "voice_recording"

	silenceThreshold := float64(1)
	silenceDuration := 2 * time.Second

	lastSoundTime := time.Now()

	canRecord := false
	isRecording := false
	silenceTimerStarted := false

	for {

		if !canRecord {
			if _, err := os.Stat("./can_record"); err != nil {
				time.Sleep(time.Second * 1)
				continue
			} else {
				canRecord = true
				recCmd = exec.Command("sh", "-c", "arecord", "-f", "wav", "-")
				recStdout, err = recCmd.StdoutPipe()
				errCheck(recCmd.Start())
			}
		}

		tempBuf := make([]byte, 1024, 1024)
		n, err := recStdout.Read(tempBuf)
		errCheck(err)

		current := tempBuf[:n]

		if isRecording {
			clip.Write(current)
		}

		if n == 44 {
			header = current
		} else if n > 0 {
			rms := calculateRMS(current)

			if rms > silenceThreshold {

				println(rms, silenceThreshold)

				if !isRecording {

					clip.Write(header)
					clip.Write(current)

					fmt.Println("Starting new recording...")

					isRecording = true
					silenceTimerStarted = false
				}

				lastSoundTime = time.Now()
			} else if isRecording && !silenceTimerStarted {
				silenceTimerStarted = true
				lastSoundTime = time.Now()
			}
		}

		if silenceTimerStarted && time.Since(lastSoundTime) >= silenceDuration {
			os.Remove("./can_record")
			recStdout.Close()

			println("Creating clip...")

			f, err := os.Create(recordingFile + ".wav")
			errCheck(err)
			f.Write(clip.Bytes())
			f.Close()

			whispercmd := exec.Command("whisper", "--output_format", "txt", recordingFile+".wav")
			err = whispercmd.Run()
			if err == nil {
				vr, err := os.ReadFile(recordingFile + ".txt")
				errCheck(err)

				text <- string(vr)
			}

			clip.Reset()

			canRecord = false
			isRecording = false
			silenceTimerStarted = false
		}
	}

}
