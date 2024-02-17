package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

const (
	server      = "irc.chat.twitch.tv"
	port        = "6667"
	redirectUri = "http://localhost:7776/auth/callback"
)

type Cred struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type User struct {
	ID string `json:"id"`
}

type UserRequest struct {
	Data []User `json:"data"`
}

type Reward struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type RewardRequest struct {
	Data []Reward `json:"data"`
}

func getTokenBody(data url.Values) {
	resp, err := http.PostForm("https://id.twitch.tv/oauth2/token", data)
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var cred Cred

	if err := json.Unmarshal(body, &cred); err != nil {
		log.Fatal(err)
	}

	if len(cred.AccessToken) > 0 {
		f, err := os.Create("access_token.txt")
		if err != nil {
			log.Fatal(err)
		}
		f.WriteString(cred.AccessToken)
		f.Close()
	}

	if len(cred.RefreshToken) > 0 {
		f, err := os.Create("refresh_token.txt")
		if err != nil {
			log.Fatal(err)
		}
		f.WriteString(cred.RefreshToken)
		f.Close()
	}
}

func getClientCred(code string) {
	data := url.Values{
		"client_id":     {os.Getenv("TWITCH_CLIENT_ID")},
		"client_secret": {os.Getenv("TWITCH_CLIENT_SECRET")},
	}

	if len(code) > 0 {
		data.Add("code", code)
		data.Add("grant_type", "authorization_code")
		data.Add("redirect_uri", redirectUri)
	} else {
		data.Add("grant_type", "client_credentials")
	}

	getTokenBody(data)
}

func getRefreshToken() {
	rt, err := os.ReadFile("refresh_token.txt")
	if err != nil {
		log.Fatal(err)
	}

	data := url.Values{
		"client_id":     {os.Getenv("TWITCH_CLIENT_ID")},
		"client_secret": {os.Getenv("TWITCH_CLIENT_SECRET")},
		"grant_type":    {"refresh_token"},
		"refresh_token": {string(rt)},
	}

	getTokenBody(data)
}

func getHelixRequest(url string) []byte {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}

	token, err := os.ReadFile("access_token.txt")
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
		getRefreshToken()
		return getHelixRequest(url)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	return body
}

func getUserId(user string) string {
	body := getHelixRequest("https://api.twitch.tv/helix/users?login=" + user)

	var userRequest UserRequest

	if err := json.Unmarshal(body, &userRequest); err != nil {
		log.Fatal(err)
	}

	return userRequest.Data[0].ID
}

func getRewards(userId string) []Reward {
	body := getHelixRequest("https://api.twitch.tv/helix/channel_points/custom_rewards?broadcaster_id=" + userId)

	var rewardRequest RewardRequest

	if err := json.Unmarshal(body, &rewardRequest); err != nil {
		log.Fatal(err)
	}

	return rewardRequest.Data
}

func main() {

	user := os.Getenv("TWITCH_USER")
	if user == "" || os.Getenv("TWITCH_CLIENT_ID") == "" || os.Getenv("TWITCH_CLIENT_SECRET") == "" {
		panic("TWITCH_USER, TWITCH_CLIENT_ID, and TWITCH_CLIENT_SECRET must be in the environment")
	}

	var wg sync.WaitGroup

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")

		if len(code) > 0 {
			getClientCred(code)
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

	go func() {
		err := http.ListenAndServe(":7776", mux)
		if errors.Is(err, http.ErrServerClosed) {
			println("server closed")
		} else if err != nil {
			println("err: %s", err)
		}
	}()

	uri := "https://id.twitch.tv/oauth2/authorize?client_id=" + os.Getenv("TWITCH_CLIENT_ID") + "&redirect_uri=" + redirectUri

	if _, err := os.Stat("access_token.txt"); errors.Is(err, os.ErrNotExist) {
		wg.Add(1)
		println("Approve redemption request: " + uri + "&response_type=code&scope=channel:read:redemptions")
	}

	wg.Wait()

	if _, err := os.Stat("chat_token.txt"); errors.Is(err, os.ErrNotExist) {
		wg.Add(2) // Change the # in the resulting url to a ?
		println("REQUIRED: After clicking the following link, change the # in the url to a ? and then press Enter on the URL bar in order to continue.")
		println(uri + "&response_type=token&scope=chat:read+chat:edit")
	}

	wg.Wait()

	chatToken, err := os.ReadFile("chat_token.txt")
	if err != nil {
		log.Fatal(err)
	}

	getClientCred("")

	userId := getUserId(user)

	if userId == "" {
		panic("no user found")
	}

	rewards := getRewards(userId)

	rewardsJson, _ := json.Marshal(rewards)

	fmt.Println(string(rewardsJson))

	conn, err := net.Dial("tcp", server+":"+port)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}

	defer conn.Close()

	fmt.Fprintf(conn, "CAP REQ :twitch.tv/membership twitch.tv/tags twitch.tv/commands\r\n")
	fmt.Fprintf(conn, "PASS oauth:%s\r\n", string(chatToken))
	fmt.Fprintf(conn, "NICK %s\r\n", user)
	fmt.Fprintf(conn, "JOIN #%s\r\n", user)

	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line)

			if strings.HasPrefix(line, "PING") {
				pongMsg := strings.Replace(line, "PING", "PONG", 1)
				fmt.Fprintf(conn, "%s\r\n", pongMsg)
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			input := scanner.Text()
			fmt.Fprintf(conn, "PRIVMSG #%s :%s\r\n", user, input)
		}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	fmt.Println("Exiting...")
}