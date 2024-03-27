package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"
)

var MATCH_PATTERN = regexp.MustCompile("^[a-zA-Z0-9_]*$") // match Test_123

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

	stopTime := startTime.Add(7 * time.Second)

	return startTime.Format(targetLayout), stopTime.Format(targetLayout), nil
}

func parseSoundName(user string, name string) string {
	return "sounds/" + user + "_" + name + ".*"
}

func parseSoundNameAlternate(name string) string {
	return "sounds/" + name + ".*"
}

func addSound(user string, name string, yturl string, timestamp string) string {

	if !MATCH_PATTERN.MatchString(name) {
		return "bad name, only letters and numbers"
	}

	patternStr := fmt.Sprintf("/%s\\.", user+"_"+name)
	patternMatcher := regexp.MustCompile(patternStr)
	soundName := parseSoundName(user, name)

	files, err := filepath.Glob(soundName)
	errCheck(err)

	for _, name := range files {
		if patternMatcher.MatchString(name) {
			return "name already in use"
		}
	}

	startTime, stopTime, err := parseTimestamp(timestamp)
	if err != nil {
		return "bad time format, mm:ss or hh:mm:ss only"
	}

	_, err = url.ParseRequestURI(yturl)
	if err != nil {
		return "bad url"
	}

	duration := "\"*" + startTime + "-" + stopTime + "\""

	ytdlpCmd := exec.Command("sh", "-c", "yt-dlp -q -x -o sounds/"+user+"_"+name+" --force-keyframes-at-cuts --download-sections "+duration+" "+yturl)
	if err := ytdlpCmd.Run(); err != nil {
		log.Fatal(err)
	}

	return name + " sound created! you can play it with !p " + name + " -- others can play it with !p " + user + "_" + name
}

func playSound(user string, name string) {

	if !MATCH_PATTERN.MatchString(name) {
		return
	}

	var soundFile string

	// First check if we want to play a different person's sound
	// like !p user_hey as opposed to !p hey, which would play
	// my own version of "hey", instead of a different user's
	soundNameAlt := parseSoundNameAlternate(name)
	altFiles, _ := filepath.Glob(soundNameAlt)
	if len(altFiles) > 0 {
		soundFile = soundNameAlt
	}

	if len(soundFile) < 1 {
		soundName := parseSoundName(user, name)
		soundFiles, _ := filepath.Glob(soundName)
		if len(soundFiles) > 0 {
			soundFile = soundName
		}
	}

	if len(soundFile) > 0 {
		aplayCmd := exec.Command("sh", "-c", "ffplay -autoexit -nodisp "+soundFile)
		if err := aplayCmd.Run(); err != nil {
			log.Fatal(err)
		}
	}
}

func deleteSound(user string, name string) {
	patternStr := fmt.Sprintf("/%s\\.", user+"_"+name)
	patternMatcher := regexp.MustCompile(patternStr)

	soundName := parseSoundName(user, name)
	files, err := filepath.Glob(soundName)
	errCheck(err)

	for _, name := range files {
		if patternMatcher.MatchString(name) {
			os.Remove(name)
		}
	}
}
