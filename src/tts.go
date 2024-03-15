package main

import (
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"time"
)

const (
	ttsModel = "tts_models/en/ljspeech/vits"
)

func speak(message string) {
	data := url.Values{}
	data.Set("text", message)

	resp, _ := http.Get("http://localhost:5002/api/tts?" + data.Encode())

	aplayCmd := exec.Command("aplay")
	aplayCmd.Stdin = resp.Body

	if err := aplayCmd.Run(); err != nil {
		log.Fatal(err)
	}
}

func startTTSServer() {
	ttsCmd := exec.Command("tts-server", "--use_cuda", "true", "--model_name", ttsModel)
	ttsErr := ttsCmd.Start()
	if ttsErr != nil {
		log.Fatal(ttsErr)
	}

	for {
		resp, err := http.Head("http://localhost:5002")
		if err == nil && resp.StatusCode == http.StatusOK {
			break
		}
		time.Sleep(5 * time.Second)
	}
}
