package main

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

var commands = map[string][]string{
	"tts":   {"tts", "t", "talk", "say", "v"},
	"ask":   {"a"},
	"queue": {"q", "queue", "queue song", "que", "song", "p", "play"},
}

func main() {

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

	user := os.Getenv("TWITCH_USER")

	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)

			if strings.HasPrefix(line, "PING") {
				pongMsg := strings.Replace(line, "PING", "PONG", 1)
				fmt.Fprintf(conn, "%s\r\n", pongMsg)
			} else if strings.Contains(line, "PRIVMSG") {

				parts := strings.SplitN(line, " PRIVMSG #awayto :", 2)

				if len(parts) != 2 {
					continue
				}

				payload := parts[0]
				message := parts[1]

				var command string

				if strings.HasPrefix(message, "!") {
					commandParts := strings.SplitN(message, " ", 2)
					command = strings.TrimPrefix(commandParts[0], "!")
					message = commandParts[1]
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

				if contains(commands["ask"], strings.ToLower(command)) {
					var cut string
					m := postCompletion(message)

					if len(m) > 500 {
						cut = m[0:499]
					} else {
						cut = m
					}

					fmt.Fprintf(conn, "PRIVMSG #%s :%s\r\n", user, cut)

					speak(m)
				}

				if contains(commands["tts"], strings.ToLower(command)) {
					println("in tts")
					speak(message)
				}

				if contains(commands["queue"], strings.ToLower(command)) {
					queued := queueSong(message)
					if len(queued) > 0 {
						fmt.Fprintf(conn, "PRIVMSG #%s :%s\r\n", user, queued)
					}
				}

			}
			// else if msgRegex.MatchString(line) {
			// }
		}
	}()

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			input := scanner.Text()
			fmt.Fprintf(conn, "PRIVMSG #%s :%s\r\n", user, input)
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
