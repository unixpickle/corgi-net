package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/unixpickle/essentials"
)

func main() {
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
	baseURL := "https://www.reddit.com/r/" + url.PathEscape(name) + "/new.json?count=100"
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
	resp, err := http.Get(u)
	fmt.Println(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
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
