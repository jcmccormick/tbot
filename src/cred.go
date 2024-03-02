package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func getServiceRedirect(service string) string {
	return "http://localhost:7776/auth/" + service + "/callback"
}

func getCreds(service string, host string, code string) {
	data := url.Values{
		"client_id":     {os.Getenv(strings.ToUpper(service) + "_CLIENT_ID")},
		"client_secret": {os.Getenv(strings.ToUpper(service) + "_CLIENT_SECRET")},
	}

	if len(code) > 0 {
		data.Add("code", code)
		data.Add("grant_type", "authorization_code")
		data.Add("redirect_uri", getServiceRedirect(service))
	} else {
		data.Add("grant_type", "client_credentials")
	}

	getTokenBody(service, host, data)
}

func refreshCreds(service string, host string) {
	rt, err := os.ReadFile(service + "_refresh_token.txt")
	if err != nil {
		log.Fatal(err)
	}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {string(rt)},
	}

	if service == "twitch" {
		data.Add("client_id", os.Getenv(strings.ToUpper(service)+"_CLIENT_ID"))
		data.Add("client_secret", os.Getenv(strings.ToUpper(service)+"_CLIENT_SECRET"))
	}

	getTokenBody(service, host, data)
}

func getTokenBody(service string, url string, data url.Values) {
	var resp *http.Response
	var err error

	if service == "spotify" {
		req, err := http.NewRequest("POST", url, strings.NewReader(data.Encode()))
		if err != nil {
			panic(err)
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(os.Getenv("SPOTIFY_CLIENT_ID")+":"+os.Getenv("SPOTIFY_CLIENT_SECRET"))))

		client := &http.Client{}
		resp, err = client.Do(req)
		if err != nil {
			panic(err)
		}
	} else {
		resp, err = http.PostForm(url, data)
		if err != nil {
			log.Fatal(err)
		}
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
		f, err := os.Create(service + "_access_token.txt")
		if err != nil {
			log.Fatal(err)
		}
		f.WriteString(cred.AccessToken)
		f.Close()
	}

	if len(cred.RefreshToken) > 0 {
		f, err := os.Create(service + "_refresh_token.txt")
		if err != nil {
			log.Fatal(err)
		}
		f.WriteString(cred.RefreshToken)
		f.Close()
	}
}
