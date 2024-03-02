package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
)

func getSpotifyResource(method string, url string) []byte {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		log.Fatal(err)
	}

	token, err := os.ReadFile("spotify_access_token.txt")
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Authorization", "Bearer "+string(token))

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode == 401 {
		refreshCreds("spotify", "https://accounts.spotify.com/api/token")
		return getSpotifyResource(method, url)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed spotify request: \n " + err.Error())
	}

	return body
}

func setupSpotifyMux(mux *http.ServeMux, wg *sync.WaitGroup) {
	mux.HandleFunc("/auth/spotify/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")

		if len(code) > 0 {
			getCreds("spotify", "https://accounts.spotify.com/api/token", code)
		}

		defer wg.Done()
	})
}

func getSong(query string) (Item, error) {
	params := url.Values{}
	params.Add("q", query)
	params.Add("type", "track")

	var tracksResponse SpotifyTracksResponse

	tracksBody := getSpotifyResource("GET", "https://api.spotify.com/v1/search?"+params.Encode())

	if err := json.Unmarshal(tracksBody, &tracksResponse); err != nil {
		log.Fatal("Couldn't read song uri", err)
	}

	if len(tracksResponse.Tracks.Items) > 0 {
		return tracksResponse.Tracks.Items[0], nil
	}

	return Item{}, errors.New("Couldn't find that song. Only use the artist and track name.")
}

func queueSong(query string) string {
	song, err := getSong(query)
	if err != nil {
		return err.Error()
	}

	params := url.Values{}
	params.Add("uri", song.Uri)

	getSpotifyResource("POST", "https://api.spotify.com/v1/me/player/queue?"+params.Encode())

	return "Queued " + song.Name + " by " + song.Artists[0].Name
}

func nextSong() {
	getSpotifyResource("POST", "https://api.spotify.com/v1/me/player/next")
}

func requestSpotifyCreds(wg *sync.WaitGroup) {
	spotifyUri := "https://accounts.spotify.com/authorize?client_id=" + os.Getenv("SPOTIFY_CLIENT_ID") + "&redirect_uri=" + getServiceRedirect("spotify")

	if _, err := os.Stat("spotify_access_token.txt"); errors.Is(err, os.ErrNotExist) {
		wg.Add(1)
		println(spotifyUri + "&response_type=code&scope=user-read-private%20user-modify-playback-state")
	}

	wg.Wait()
}
