package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/unixpickle/essentials"
)

const MinResolution = 512

var OutputDir = "images"

func main() {
	os.Mkdir(OutputDir, 0755)
	links := GetImageLinks("../list_pushshift/listing")
	sort.Slice(links, func(i, j int) bool {
		return strings.Compare(links[i].URL, links[j].URL) == -1
	})
	log.Printf("total of %d URLs found", len(links))
	numDownloaded := 0
	numErrors := 0
	index := map[string]*ImageLink{}
	for _, link := range links {
		urlHash := md5.Sum([]byte(link.URL))
		urlStr := hex.EncodeToString(urlHash[:])
		outName := filepath.Join(OutputDir, urlStr+".jpg")
		errorName := filepath.Join(OutputDir, urlStr+"_error.txt")
		if _, err := os.Stat(outName); err == nil {
			index[urlStr] = link
			numDownloaded++
			continue
		}
		if _, err := os.Stat(errorName); err == nil {
			numErrors++
			continue
		}
		endTime := time.Now().Add(time.Second)
		imageData, err := DownloadImage(link.URL)
		if err != nil {
			essentials.Must(ioutil.WriteFile(errorName, []byte(err.Error()), 0644))
			numErrors++
		} else {
			index[urlStr] = link
			essentials.Must(ioutil.WriteFile(outName, imageData, 0644))
			numDownloaded++
		}
		log.Printf("downloaded %d (got %d errors)", numDownloaded, numErrors)
		time.Sleep(time.Until(endTime))
	}
	indexData, err := json.Marshal(index)
	essentials.Must(err)
	essentials.Must(ioutil.WriteFile("index.json", indexData, 0644))
}

func DownloadImage(u string) ([]byte, error) {
	var resp *http.Response
	for i := 0; i < 3; i++ {
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
	if resp == nil {
		return nil, errors.New("too many rate limit responses")
	}
	decoded, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, err
	}
	bounds := decoded.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			out.Set(x, y, decoded.At(x+bounds.Min.X, y+bounds.Min.Y))
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 100}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func GetImageLinks(containerDir string) []*ImageLink {
	listing, err := ioutil.ReadDir(containerDir)
	essentials.Must(err)
	var links []*ImageLink
	for _, item := range listing {
		if strings.HasPrefix(item.Name(), ".") || !strings.HasSuffix(item.Name(), ".json") {
			continue
		}
		data, err := ioutil.ReadFile(filepath.Join(containerDir, item.Name()))
		essentials.Must(err)
		var results ListResult
		essentials.Must(json.Unmarshal(data, &results))
		for _, entry := range results.Data {
			if !entry.Indexable {
				// Post was likely removed due to moderation.
				continue
			}

			link := &ImageLink{
				Title:      entry.Title,
				CreatedUTC: entry.CreatedUTC,
				Permalink:  entry.Permalink,
			}

			if entry.Preview != nil {
				// Find the smallest preview to save bandwidth.
				var smallest int
				var smallestURL string
				for _, preview := range entry.Preview.Images {
					if preview.Resolutions == nil {
						continue
					}
					for _, res := range append(*preview.Resolutions, *preview.Source) {
						size := essentials.MinInt(res.Width, res.Height)
						if size > MinResolution && (smallest == 0 || size < smallest) {
							smallestURL = res.URL
							smallest = size
						}
					}
				}
				if smallestURL != "" {
					link.URL = html.UnescapeString(smallestURL)
					links = append(links, link)
					continue
				}
			}

			// Fall-back to raw post image if available.
			if strings.HasPrefix(entry.URL, "https://i.redd.it/") && strings.HasSuffix(entry.URL, ".jpg") {
				link.URL = entry.URL
				links = append(links, link)
				continue
			}
		}

	}
	return links
}

type ImageLink struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	CreatedUTC int64  `json:"created_utc"`
	Permalink  string `json:"permalink"`
}

type ListResult struct {
	Data []struct {
		URL        string `json:"url"`
		Indexable  bool   `json:"is_robot_indexable"`
		Title      string `json:"title"`
		CreatedUTC int64  `json:"created_utc"`
		Permalink  string `json:"permalink"`
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
