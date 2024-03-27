package main

import (
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func parseTimestamp(timestamp string) (string, string, error) {
	var targetLayout string
	layouts := []string{
		"15:04:05",
		"04:05",
	}

	var startTime time.Time
	var err error

	for _, layout := range layouts {
		startTime, err = time.Parse(layout, timestamp)
		if err == nil {
			targetLayout = layout
			break
		}
	}

	if err != nil {
		return "", "", err
	}

	stopTime := startTime.Add(5 * time.Second)

	return startTime.Format(targetLayout), stopTime.Format(targetLayout), nil
}

func parseSoundName(user string, name string) string {
	return "sounds/" + user + "_" + name + ".wav"
}

func addSound(user string, name string, yturl string, timestamp string) string {
	soundName := parseSoundName(user, name)
	if _, err := os.Stat(soundName); err == nil {
		return "name already in use"
	}

	startTime, stopTime, err := parseTimestamp(timestamp)
	if err != nil {
		return "bad time format, mm:ss or hh:mm:ss only"
	}

	duration := "\"*" + startTime + "-" + stopTime + "\""

	_, err = url.ParseRequestURI(yturl)
	if err != nil {
		return "bad url"
	}

	ytdlpCmd := exec.Command("sh", "-c", "yt-dlp -q -x -o sounds/ytsound --force-keyframes-at-cuts --download-sections "+duration+" "+yturl)
	if err := ytdlpCmd.Run(); err != nil {
		log.Fatal(err)
	}

	ffmpegCmd := exec.Command("sh", "-c", "ffmpeg -i sounds/ytsound.* "+soundName)
	if err := ffmpegCmd.Run(); err != nil {
		log.Fatal(err)
	}

	files, err := filepath.Glob("sounds/ytsound.*")
	errCheck(err)

	for _, file := range files {
		os.Remove(file)
	}

	return name + " sound created, play it with !p " + name
}

func playSound(user string, name string) {
	soundName := parseSoundName(user, name)
	if _, err := os.Stat(soundName); err != nil {
		return
	}

	aplayCmd := exec.Command("sh", "-c", "aplay "+soundName)
	if err := aplayCmd.Run(); err != nil {
		log.Fatal(err)
	}
}

func deleteSound(user string, name string) {
	soundName := parseSoundName(user, name)
	if _, err := os.Stat(soundName); err == nil {
		os.Remove(soundName)
	}
}
