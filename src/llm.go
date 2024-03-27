package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
)

const (
	modModel = "tbot-mod"
	llmModel = "mistral"
)

// "context"

// "regexp"

// "github.com/tmc/langchaingo/llms"
// "github.com/tmc/langchaingo/llms/ollama"

// msgRegex := regexp.MustCompile("PRIV1MSG|JO1IN|PA1RT")

// llm, err := ollama.New(ollama.WithModel(llmModel))
//
//	if err != nil {
//		log.Fatal(err)
//	}
//
// response := ""
// dataSet := "{}"
//
//	streamingFn := llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
//		response += string(chunk)
//		if len(chunk) == 0 {
//			dataSet = response
//			response = ""
//		}
//		return nil
//	})

type BasicOutput struct {
	Output string `json:"output"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMOpts struct {
	// NumCtx     int      `json:"num_ctx,omitempty"`
	NumPredict int `json:"num_predict"`
	// Stop       []string `json:"stop,omitempty"`
}

var LLMOptDefaults = LLMOpts{
	// NumCtx: 2046,
	NumPredict: 75,
	// Stop: []string{" ", "[INST]"},
}

type LLMChatRequest struct {
	Messages []Message `json:"messages"`
	// Format   string    `json:"format"`
	// System  string  `json:"system"`
	Model     string  `json:"model"`
	Stream    bool    `json:"stream"`
	Options   LLMOpts `json:"options"`
	KeepAlive int     `json:"keep_alive"`
}

var LLMChatRequestDefaults = LLMChatRequest{
	Messages: []Message{},
	// Format:   "",
	// System:  "",
	Model:     "",
	Stream:    false,
	Options:   LLMOptDefaults,
	KeepAlive: 0,
}

type LLMGenerateRequest struct {
	Prompt string `json:"prompt"`
	// Format  string  `json:"format"`
	System  string  `json:"system"`
	Model   string  `json:"model"`
	Stream  bool    `json:"stream"`
	Options LLMOpts `json:"options"`
	// KeepAlive string `json:"keep_alive"`
}

var LLMGenerateRequestDefaults = LLMGenerateRequest{
	Prompt: "",
	// Format:  "",
	System:  "",
	Model:   "",
	Stream:  false,
	Options: LLMOptDefaults,
}

type LLMResponse struct {
	Message  Message `json:"message"`
	Response string  `json:"response"`
	Context  []int   `json:"context"`
	Error    string  `json:"error"`
}

func getBetween(str string, start string, end string) (result string) {

	s := strings.Index(str, start)
	if s == -1 {
		return
	}

	e := strings.Index(str, end)
	if e == -1 {
		return
	}

	println("got string: " + str[s:e+1])

	return str[s : e+1]
}

func getCompletion(prompt string) {

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
}

func GenerateCompletion(data LLMGenerateRequest, resString *[]byte) error {

	jsonData, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest(http.MethodPost, "http://localhost:11434/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		panic(err)
	}

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

	var llmResponse LLMResponse

	if err := json.Unmarshal(body, &llmResponse); err != nil {
		log.Fatal(err)
	}

	if len(llmResponse.Error) > 0 {
		panic(llmResponse.Error)
	}

	*resString = []byte(llmResponse.Response)

	return nil
}

func GenerateChat(data LLMChatRequest, resString *[]byte) error {

	jsonData, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}

	// j, _ := json.MarshalIndent(data, "", "    ")
	//
	// println(string(j))

	req, err := http.NewRequest(http.MethodPost, "http://localhost:11434/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		panic(err)
	}
	// req.Header.Set("Content-Type", "application/json")

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

	var llmResponse LLMResponse

	if err := json.Unmarshal(body, &llmResponse); err != nil {
		log.Fatal(err)
	}

	*resString = []byte(llmResponse.Message.Content)

	return nil
}

func getModeration(message string) string {
	var moderationEvent BasicOutput

	var res []byte

	// completion := LLMGenerateRequest{}
	//
	// opts := LLMOpts{
	// 	NumCtx: LLMOptDefaults.NumCtx,
	// 	Stop:   LLMOptDefaults.Stop,
	// }

	GenerateCompletion(LLMGenerateRequestDefaults, &res)

	if err := json.Unmarshal(res, &moderationEvent); err != nil {
		log.Fatal(err)
	}

	return moderationEvent.Output
}
