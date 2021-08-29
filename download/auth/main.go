package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/unixpickle/essentials"
)

func main() {
	var clientID string
	var clientSecret string
	var username string
	var password string
	flag.StringVar(&clientID, "client-id", "", "reddit API client ID")
	flag.StringVar(&clientSecret, "client-secret", "", "reddit API client secret")
	flag.StringVar(&username, "username", "", "reddit account username")
	flag.StringVar(&password, "password", "", "reddit account password")
	flag.Parse()

	query := url.Values{}
	query.Set("username", username)
	query.Set("password", password)
	query.Set("grant_type", "password")
	body := bytes.NewReader([]byte(query.Encode()))
	req, err := http.NewRequest("POST", "https://www.reddit.com/api/v1/access_token", body)
	essentials.Must(err)
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("User-Agent", "corgi-downloader-v1 (by unixpickle)")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	essentials.Must(err)
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	essentials.Must(err)

	fmt.Println(string(data))
}
