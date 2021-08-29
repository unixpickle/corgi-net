package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/unixpickle/essentials"
)

func main() {
	os.Mkdir("listing", 0755)

	it := IterateSubreddit("corgi")
	total := 0
	for i := 0; true; i++ {
		listing, err := it()
		if err == io.EOF {
			break
		}
		essentials.Must(err)
		data, err := json.Marshal(listing)
		essentials.Must(err)
		essentials.Must(ioutil.WriteFile(fmt.Sprintf("listing/%05d.json", i), data, 0644))
		total += len(listing.Data)
		log.Printf("downloaded %d records", total)
	}
}

func IterateSubreddit(name string) func() (*ListResult, error) {
	baseURL := "https://api.pushshift.io/reddit/search/submission/?subreddit=" + url.QueryEscape(name) + "&size=500"
	var listing *ListResult
	return func() (*ListResult, error) {
		u := baseURL
		if listing != nil {
			u += "&before=" + strconv.FormatInt(listing.Data[len(listing.Data)-1].CreatedUTC, 10)
		}
		var err error
		listing, err = fetchListRequest(u)
		if err == nil && len(listing.Data) == 0 {
			return nil, io.EOF
		}
		return listing, err
	}
}

func fetchListRequest(u string) (*ListResult, error) {
	var resp *http.Response
	for i := 0; true; i++ {
		var err error
		resp, err = http.Get(u)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 429 {
			resp.Body.Close()
			resp = nil
			backoff := time.Second * time.Duration(1<<uint(i))
			log.Printf("rate limited; backing off for %v", backoff)
			time.Sleep(backoff)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("bad HTTP status code: %d", resp.StatusCode)
		}
		break
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
	Data []struct {
		Title      string `json:"title"`
		URL        string `json:"url"`
		Thumbnail  string `json:"thumbnail"`
		Indexable  bool   `json:"is_robot_indexable"`
		CreatedUTC int64  `json:"created_utc"`
		Permalink  string `json:"permalink"`
		FullLink   string `json:"full_link"`
		Preview    *struct {
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
}
