package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/unixpickle/essentials"
)

var (
	AccessToken string
	UserAgent   string
)

func main() {
	flag.StringVar(&AccessToken, "access-token", "", "reddit API access token")
	flag.StringVar(&UserAgent, "user-agent", "corgi-downloader-v1 (by unixpickle)",
		"HTTP user agent")
	flag.Parse()
	if AccessToken == "" {
		essentials.Die("Missing required -access-token argument.")
	}

	os.Mkdir("listing", 0755)

	it := IterateSubreddit("corgi")
	for i := 0; true; i++ {
		listing, err := it()
		essentials.Must(err)
		data, err := json.Marshal(listing)
		essentials.Must(err)
		essentials.Must(ioutil.WriteFile(fmt.Sprintf("listing/%05d.json", i), data, 0644))
		time.Sleep(time.Second * 5)
	}
}

func IterateSubreddit(name string) func() (*ListResult, error) {
	baseURL := "https://oauth.reddit.com/r/" + url.PathEscape(name) + "/new.json?limit=100"
	var listing *ListResult
	return func() (*ListResult, error) {
		u := baseURL
		if listing != nil {
			u += "&after=" + url.QueryEscape(listing.Data.After)
		}
		var err error
		listing, err = fetchListRequest(u)
		return listing, err
	}
}

func fetchListRequest(u string) (*ListResult, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+AccessToken)
	req.Header.Set("User-Agent", UserAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, _ := ioutil.ReadAll(resp.Body)
		fmt.Println(string(data))
		return nil, fmt.Errorf("bad HTTP status code: %d", resp.StatusCode)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var obj ListResult
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

type ListResult struct {
	Data struct {
		After    string `json:"after"`
		Children []struct {
			Kind string `json:"kind"`
			Data struct {
				Title     string `json:"title"`
				Downs     int    `json:"downs"`
				Ups       int    `json:"ups"`
				Thumbnail string `json:"thumbnail"`
				Preview   *struct {
					Images []struct {
						Source *struct {
							URL    string `json:"url"`
							Width  int    `json:"width"`
							Height int    `json:"height"`
						} `json:"source"`
						Resolutions *[]struct {
							URL    string `json:"url"`
							Width  int    `json:"width"`
							Height int    `json:"height"`
						} `json:"resolutions"`
					} `json:"images"`
				} `json:"preview"`
			} `json:"data"`
		} `json:"children"`
	} `json:"data"`
}
