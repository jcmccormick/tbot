package main

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

var commands = map[string][]string{
	"tts":        {"tts", "t", "talk", "say"},
	"ask":        {"a"},
	"queue":      {"q", "queue"},
	"skip":       {"s", "skip"},
	"clear":      {"c", "clear"},
	"add_sound":  {"as", "add_sound"},
	"play_sound": {"p", "play", "play_sound"},
	"del_sound":  {"ds", "del_sound"},
}

func main() {
	user := os.Getenv("TWITCH_USER")

	convos := Convos{
		Chatters: make(map[string]Convo),
	}
	spokenTexts := make(chan string)

	chatRequest := LLMChatRequestDefaults
	chatRequest.Options.NumPredict = 150
	chatRequest.Model = "tbot-chat"

	var wg sync.WaitGroup

	mux := http.NewServeMux()

	setupTwitchMux(mux, &wg)
	setupSpotifyMux(mux, &wg)

	go func() {
		err := http.ListenAndServe(":7776", mux)
		if errors.Is(err, http.ErrServerClosed) {
			println("server closed")
		} else if err != nil {
			println("err: %s", err)
		}
	}()

	requestTwitchCreds(&wg)
	requestSpotifyCreds(&wg)

	rewards := getRewards()

	startTTSServer()

	conn := getIRCConnection()

	defer conn.Close()

	connectToChannel(conn)

	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)

			if strings.HasPrefix(line, "PING") {
				pongMsg := strings.Replace(line, "PING", "PONG", 1)
				fmt.Fprintf(conn, "%s\r\n", pongMsg)
			} else if strings.Contains(line, "PRIVMSG") {

				parts := strings.SplitN(line, " PRIVMSG #"+user+" :", 2)

				if len(parts) != 2 {
					continue
				}

				payload := parts[0]
				message := parts[1]

				if len(message) == 0 {
					continue
				}

				// if getModeration(message) == "BAD" {
				// 	println("MODERATED EVENT: " + line)
				// 	continue
				// }

				var command string

				if strings.HasPrefix(message, "!") {
					commandParts := strings.SplitN(message, " ", 2)
					command = strings.TrimPrefix(commandParts[0], "!")
					if len(commandParts) > 1 {
						message = commandParts[1]
					}
				}

				contents := make(map[string]string)
				payloadItems := strings.Split(payload[1:], ";")

				for _, item := range payloadItems {
					keyValue := strings.SplitN(item, "=", 2)
					key := keyValue[0]
					value := keyValue[1]
					contents[toCamelCase(key)] = value
				}

				rewardId, hasReward := contents["customRewardId"]
				if hasReward {
					var currentReward Reward

					for _, reward := range rewards {
						if reward.ID == rewardId {
							currentReward = reward
						}
					}
					command = currentReward.Title
				}

				chatter := contents["displayName"]
				command = strings.ToLower(command)

				convos.AddUser(chatter)

				// if chatter == user {
				//
				// }
				if contains(commands["add_sound"], strings.ToLower(command)) {
					soundParts := strings.Split(message, " ")
					if len(soundParts) < 3 {
						continue
					}
					result := addSound(chatter, soundParts[0], soundParts[1], soundParts[2])
					sendToChannel(conn, user, result)
				}

				if contains(commands["play_sound"], strings.ToLower(command)) {
					go playSound(chatter, message)
				}

				if contains(commands["del_sound"], strings.ToLower(command)) {
					go deleteSound(user, message)
				}

				if contains(commands["ask"], strings.ToLower(command)) {
					convos.AddMessage(chatter, Message{Role: "user", Content: message})

					var res []byte

					chatRequest.Messages = convos.Chatters[chatter].Messages
					GenerateChat(chatRequest, &res)

					response := string(res)

					idx := strings.LastIndex(response, ".")
					if idx > -1 {
						response = response[:idx]
					}

					convos.AddMessage(chatter, Message{Role: "assistant", Content: response})
					sendToChannel(conn, user, response)
					spokenTexts <- response
				}

				if contains(commands["tts"], command) {
					convos.AddMessage(chatter, Message{Role: "user", Content: message})
					spokenTexts <- message
				}

				if contains(commands["queue"], command) {
					queued := queueSong(message)
					if len(queued) > 0 {
						sendToChannel(conn, user, queued)
					}
				}

				if contains(commands["clear"], command) {
					convos.ClearMessages(chatter)
				}

				if contains(commands["skip"], command) {
					nextSong()
				}
			}
			// else if msgRegex.MatchString(line) {
			// }
		}
	}()

	// go func() {
	// 	scanner := bufio.NewScanner(os.Stdin)
	// 	for scanner.Scan() {
	// 		input := scanner.Text()
	// 		sendToChannel(conn, user, input)
	// 	}
	// }()

	// everyone talks and feeds into the history,
	//
	var speechCmd *exec.Cmd
	var err error

	go func() {
		for spokenText := range spokenTexts {
			speechCmd, err = speak(spokenText)
			errCheck(err)
		}
	}()

	voiceTexts := make(chan string)

	go GetTextFromSpeech(voiceTexts)
	go func() {
		for voiceText := range voiceTexts {
			print("SPOKEN WORD:", voiceText)

			text := strings.ToLower(voiceText)

			if strings.Contains(text, "stop") {
				err := speechCmd.Process.Kill()
				errCheck(err)
			}

			if strings.Contains(text, "clear") {
				if strings.Contains(text, "history") {
					convos.ClearMessages(user)
				}
			}

			convos.AddMessage(user, Message{Role: "user", Content: text})

			var res []byte

			chatRequest.Messages = convos.Chatters[user].Messages
			chatRequest.Options.NumPredict = 200
			GenerateChat(chatRequest, &res)

			response := string(res)

			idx := strings.LastIndex(response, ".")
			if idx > -1 {
				response = response[:idx]
			}

			convos.AddMessage(user, Message{Role: "assistant", Content: response})

			spokenTexts <- response
		}
	}()

	// ticker := time.NewTicker(30 * time.Minute)
	// quit := make(chan struct{})
	// go func() {
	// 	for {
	// 		select {
	// 		case <-ticker.C:
	// 			refreshCreds("spotify", "https://accounts.spotify.com/api/token")
	// 			refreshCreds("twitch", "https://id.twitch.tv/oauth2/token")
	// 		case <-quit:
	// 			ticker.Stop()
	// 			return
	// 		}
	// 	}
	// }()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	// close(quit)

	fmt.Println("Exiting...")
}
