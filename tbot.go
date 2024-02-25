package main

import (
	"bufio"
	"bytes"

	// "context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"

	// "regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	// "github.com/tmc/langchaingo/llms"
	// "github.com/tmc/langchaingo/llms/ollama"
)

const (
	server      = "irc.chat.twitch.tv:6667"
	ttsModel    = "tts_models/en/ljspeech/vits"
	llmModel    = "mistral"
	redirectUri = "http://localhost:7776/auth/callback"
)

var commands = map[string][]string{
	"tts": {"tts", "t", "talk", "say", "v"},
}

var adminCommands = map[string][]string{
	"ask": {"a"},
}

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

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatMessage struct {
	Content string `json:"content"`
}

type ChatRequest struct {
	Message ChatMessage `json:"message"`
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

func toCamelCase(input string) string {
	parts := strings.Split(input, "-")
	for i, part := range parts {
		if i > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

func contains(a []string, s string) bool {
	for _, e := range a {
		if e == s {
			return true
		}
	}
	return false
}

func main() {

	user := os.Getenv("TWITCH_USER")
	if user == "" || os.Getenv("TWITCH_CLIENT_ID") == "" || os.Getenv("TWITCH_CLIENT_SECRET") == "" {
		panic("TWITCH_USER, TWITCH_CLIENT_ID, and TWITCH_CLIENT_SECRET must be in the environment")
	}

	var wg sync.WaitGroup

	// llm, err := ollama.New(ollama.WithModel(llmModel))
	// if err != nil {
	// 	log.Fatal(err)
	// }
	//
	// response := ""
	// dataSet := "{}"
	//
	// streamingFn := llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
	// 	response += string(chunk)
	// 	if len(chunk) == 0 {
	// 		dataSet = response
	// 		response = ""
	// 	}
	// 	return nil
	// })

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

	conn, err := net.Dial("tcp", server)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}

	defer conn.Close()

	fmt.Fprintf(conn, "CAP REQ :twitch.tv/membership twitch.tv/tags twitch.tv/commands\r\n")
	fmt.Fprintf(conn, "PASS oauth:%s\r\n", string(chatToken))
	fmt.Fprintf(conn, "NICK %s\r\n", user)
	fmt.Fprintf(conn, "JOIN #%s\r\n", user)

	// msgRegex := regexp.MustCompile("PRIV1MSG|JO1IN|PA1RT")

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

				if contents["displayName"] == os.Getenv("TWITCH_USER") {

					if contains(adminCommands["ask"], strings.ToLower(command)) {

						data := struct {
							Messages []Message `json:"messages"`
							Model    string    `json:"model"`
							Stream   bool      `json:"stream"`
						}{
							Messages: []Message{
								// {Role: "system", Content: "Maximum response length: 20 words. Please provide query."},
								{Role: "system", Content: "All responses will be minimally worded. Please provide query."},
								{Role: "user", Content: message},
							},
							Model:  "mistral",
							Stream: false,
						}

						jsonData, err := json.Marshal(data)
						if err != nil {
							panic(err)
						}

						req, err := http.NewRequest("POST", "http://localhost:11434/api/chat", bytes.NewBuffer(jsonData))
						if err != nil {
							panic(err)
						}
						req.Header.Set("Content-Type", "application/json")

						client := &http.Client{}
						resp, err := client.Do(req)
						if err != nil {
							panic(err)
						}
						defer resp.Body.Close()

						body, err := io.ReadAll(resp.Body)
						if err != nil {
							log.Fatal(err)
						}

						var chatRequest ChatRequest

						if err := json.Unmarshal(body, &chatRequest); err != nil {
							log.Fatal(err)
						}

						m := chatRequest.Message.Content

						fmt.Fprintf(conn, "PRIVMSG #%s :%s\r\n", user, m)

						speak(m)
					}

				}

				if contains(commands["tts"], strings.ToLower(command)) {
					data := url.Values{}
					data.Set("text", message)

					resp, _ := http.Get("http://localhost:5002/api/tts?" + data.Encode())

					aplayCmd := exec.Command("aplay")
					aplayCmd.Stdin = resp.Body
					if err := aplayCmd.Run(); err != nil {
						log.Fatal(err)
					}

				}

			}
			// else if msgRegex.MatchString(line) {
			// 	prompt := "Respond only with a valid JSON object representing the cumulative information received from any messages given after this prompt. Your response will be immediately parsed with json.Unmarshal into a generic Golang interface; therefore it is imperative that your response is only valid JSON. The JSON response is an object tracking the messages coming from an IRC chatroom. We need to know who is currently in the room and how many messages they have sent."
			// 	prompt += " The current JSON object is: " + dataSet
			// 	prompt += " And the next message to process is: " + line
			//
			// 	llmCtx := context.Background()
			//
			// 	completion, err := llms.GenerateFromSinglePrompt(llmCtx, llm, prompt, llms.WithTemperature(0.8), streamingFn)
			// 	if err != nil {
			// 		log.Fatal(err)
			// 	}
			//
			// 	_ = completion
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

	// ticker := time.NewTicker(30 * time.Second)
	// quit := make(chan struct{})
	// go func() {
	// 	for {
	// 		select {
	// 		case <-ticker.C:
	// 			println(dataSet)
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
