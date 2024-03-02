package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
)

const (
	ircServer = "irc.chat.twitch.tv:6667"
)

func getHelixRequest(url string) []byte {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}

	token, err := os.ReadFile("twitch_access_token.txt")
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Authorization", "Bearer "+string(token))
	req.Header.Set("Client-Id", os.Getenv("TWITCH_CLIENT_ID"))
	req.Header.Set("Accept", "application/vnd.twitchtv.v5+json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode == 401 {
		refreshCreds("twitch", "https://id.twitch.tv/oauth2/token")
		return getHelixRequest(url)
	}

	defer resp.Body.Close()

	println(url, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed helix request: \n " + err.Error())
	}

	return body
}

func getUserId(user string) string {
	body := getHelixRequest("https://api.twitch.tv/helix/users?login=" + user)

	var userRequest TwitchUserData

	if err := json.Unmarshal(body, &userRequest); err != nil {
		panic("Failed to get twitch user id: \n" + err.Error())
	}

	return userRequest.Data[0].ID
}

func getRewards() []Reward {
	userId := getUserId(os.Getenv("TWITCH_USER"))

	if userId == "" {
		panic("no user found")
	}
	body := getHelixRequest("https://api.twitch.tv/helix/channel_points/custom_rewards?broadcaster_id=" + userId)

	var rewardRequest RewardRequest

	if err := json.Unmarshal(body, &rewardRequest); err != nil {
		panic("Failed to get rewards: \n" + err.Error())
	}

	return rewardRequest.Data
}

func setupTwitchMux(mux *http.ServeMux, wg *sync.WaitGroup) {
	mux.HandleFunc("/auth/twitch/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")

		if len(code) > 0 {
			getCreds("twitch", "https://id.twitch.tv/oauth2/token", code)
		} else {
			f, err := os.Create("chat_token.txt")
			if err != nil {
				log.Fatal(err)
			}
			f.WriteString(r.URL.Query().Get("access_token"))
			f.Close()
		}

		defer wg.Done()
	})
}

func requestTwitchCreds(wg *sync.WaitGroup) {
	user := os.Getenv("TWITCH_USER")
	if user == "" || os.Getenv("TWITCH_CLIENT_ID") == "" || os.Getenv("TWITCH_CLIENT_SECRET") == "" {
		panic("TWITCH_USER, TWITCH_CLIENT_ID, and TWITCH_CLIENT_SECRET must be in the environment")
	}

	twitchUri := "https://id.twitch.tv/oauth2/authorize?client_id=" + os.Getenv("TWITCH_CLIENT_ID") + "&redirect_uri=" + getServiceRedirect("twitch")

	if _, err := os.Stat("twitch_access_token.txt"); errors.Is(err, os.ErrNotExist) {
		wg.Add(1)
		println("Approve redemption request: " + twitchUri + "&response_type=code&scope=channel:read:redemptions")
	}

	wg.Wait()

	if _, err := os.Stat("chat_token.txt"); errors.Is(err, os.ErrNotExist) {
		wg.Add(2) // Change the # in the resulting url to a ?
		println("REQUIRED: After clicking the following link, change the # in the url to a ? and then press Enter on the URL bar in order to continue.")
		println(twitchUri + "&response_type=token&scope=chat:read+chat:edit")
	}

	wg.Wait()

	getCreds("twitch", "https://id.twitch.tv/oauth2/token", "")
}

func getIRCConnection() net.Conn {
	conn, err := net.Dial("tcp", ircServer)
	if err != nil {
		panic("Failed to connect: \n" + err.Error())
	}

	return conn
}

func connectToChannel(conn net.Conn) {
	user := os.Getenv("TWITCH_USER")
	chatToken, err := os.ReadFile("chat_token.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(conn, "CAP REQ :twitch.tv/membership twitch.tv/tags twitch.tv/commands\r\n")
	fmt.Fprintf(conn, "PASS oauth:%s\r\n", string(chatToken))
	fmt.Fprintf(conn, "NICK %s\r\n", user)
	fmt.Fprintf(conn, "JOIN #%s\r\n", user)
}
