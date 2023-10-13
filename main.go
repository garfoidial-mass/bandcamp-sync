package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"regexp"

	"golang.org/x/net/html"
)

var defaultSettingsJson string = `{
	"downloads":{},
	"format":"mp3-v0",
	"identity":"",
	"music_path":"",
}`
var settings map[string]interface{} //Global map containing identity token, music path, format to save the music in, and what is currently downloaded
// Retrieves settings from file and places them into a global map[string]interface{}
func getSettings() error {
	if _, err := os.Stat("settings.json"); os.IsNotExist(err) {
		os.WriteFile("settings.json", []byte(defaultSettingsJson), 0777)
	}
	settingsBuffer, err := os.ReadFile("settings.json")
	if err != nil {
		return err
	}
	settings = make(map[string]interface{})
	err = json.Unmarshal(settingsBuffer, &settings)

	for _, v := range settings {
		if v == nil {
			return errors.New("Please fill out settings.json before running again")
		}
	}

	return err
}

// Saves settings with readable formatting back to JSON file
func saveSettings() {
	settingsBuf, err := json.MarshalIndent(settings, "", "	")
	if err != nil {
		log.Fatalf("Error %s marshaling settings", err.Error())
	}
	os.WriteFile("settings.json", settingsBuf, 0777)
}

// Makes a call to the bandcamp api with the users identity token.
func apiCall(client *http.Client, funcPath string, method string, body map[string]interface{}) (map[string]interface{}, error) {
	var databuf *bytes.Buffer = nil
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var collectionReq *http.Request
	if body != nil {
		databuf = bytes.NewBuffer(data)
		collectionReq, err = http.NewRequest(method, "https://bandcamp.com/api/"+funcPath, databuf)
	} else {
		collectionReq, err = http.NewRequest(method, "https://bandcamp.com/api/"+funcPath, nil)
	}
	if err != nil {
		return nil, err
	}
	identityCookie := http.Cookie{
		Name:   "identity",
		Value:  settings["identity"].(string),
		MaxAge: 300,
	}
	collectionReq.AddCookie(&identityCookie)
	if body != nil {
		collectionReq.Header.Add("Content-Type", "application/json")
	}
	resp, err := client.Do(collectionReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	resultBlob := make(map[string]interface{}, 0)
	err = json.NewDecoder(resp.Body).Decode(&resultBlob)
	return resultBlob, nil
}

type htmlHandler func(*html.Tokenizer) interface{}

type simplifiedItem struct {
	itemID        int
	redownloadUrl string
	itemType      string
}

func parseHtml(data io.Reader, handler htmlHandler) interface{} {
	tokenizer := html.NewTokenizer(data)
Tokenloop:
	for {
		tokenType := tokenizer.Next()
		//fmt.Println(tokenType.String())
		switch tokenType.String() {
		case "Error":
			err := tokenizer.Err()
			if err == io.EOF {
				fmt.Println("Eof")
				break Tokenloop
			}
			fmt.Println("Error", err.Error(), "tokenizing html")
		case "StartTag":
			ret := handler(tokenizer)
			if ret != nil {
				return ret
			}
		}
	}
	return nil
}

func getPageData(body io.Reader) map[string]interface{} {
	return parseHtml(body, func(tokenizer *html.Tokenizer) interface{} {
		blobTmp := make(map[string]interface{}, 0)
		name, attr := tokenizer.TagName()
		if string(name) == "div" && attr == true {
			getBlob := false
			for {
				fmt.Println("searching for pagedata")
				key, val, moreattr := tokenizer.TagAttr()
				if string(key) == "id" && string(val) == "pagedata" {
					getBlob = true
				}
				if getBlob && string(key) == "data-blob" {
					fmt.Println("getting blob")
					err := json.Unmarshal(val, &blobTmp)
					if err != nil {
						log.Fatalf("Error %s unmarshaling blob", err.Error())
					}
					return blobTmp
				}
				if !moreattr {
					break
				}
			}
		}
		return nil
	}).(map[string]interface{})
}

func scrapeDownload(link string, client *http.Client) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return nil, err
	}
	identity := http.Cookie{
		Name:   "identity",
		Value:  settings["identity"].(string),
		MaxAge: 300,
	}
	req.AddCookie(&identity)
	downloadPageResp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer downloadPageResp.Body.Close()

	return getPageData(downloadPageResp.Body), nil
}

func downloadFile(client *http.Client, link string, filename string, filetype string) (*os.File, error) {
	fmt.Println("downloadFile: ", link)
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return nil, err
	}
	identity := http.Cookie{
		Name:   "identity",
		Value:  settings["identity"].(string),
		MaxAge: 300,
	}
	req.AddCookie(&identity)
	fileResp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	defer fileResp.Body.Close()
	ext := "zip"
	var outFile *os.File = nil
	if filetype == "t" {
		switch settings["format"] {
		case "aac-hi":
			ext = "m4a"
		case "aiff-lossless":
			ext = "aiff"
		case "alac":
			ext = "m4a"
		case "flac":
			ext = "flac"
		case "mp3-320":
			fallthrough
		case "mp3-v0":
			ext = "mp3"
		case "vorbis":
			ext = "ogg"
		case "wav":
			ext = "wav"
		}
		outFile, err = os.Create(settings["music_path"].(string) + filename + "." + ext)
	} else {
		outFile, err = os.Create(settings["music_path"].(string) + filename + ".zip")
	}
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(outFile, fileResp.Body)
	if err != nil {
		return nil, err
	}
	return outFile, err
}

var fileNameRegex *regexp.Regexp

func makeFileName(name string) string {
	return fileNameRegex.ReplaceAllString(name, "")
}

func main() {
	fmt.Println("Bandcamp Library Synchronizer")
	fileNameRegex = regexp.MustCompile("\\\\|/|:|\\*|\\?|<|>|\\\"")
	err := getSettings()
	if err != nil {
		log.Fatalf("Error %s loading settings", err.Error())
	}

	idjar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatalf("Error %s creating cookie jar", err.Error())
	}
	client := http.Client{
		Jar: idjar,
	}

	summaryBlob, err := apiCall(&client, "fan/2/collection_summary", "GET", nil)
	if err != nil {
		log.Fatalf("Error %s getting summary", err.Error())
	}
	collectionSummary := summaryBlob["collection_summary"].(map[string]interface{})

	userID := int64(collectionSummary["fan_id"].(float64))
	collectionBlob, err := apiCall(&client, "fancollection/1/search_items", "POST",
		map[string]interface{}{
			"fan_id":      userID,
			"search_key":  "",
			"search_type": "collection",
		})
	if err != nil {
		log.Fatalf("Error %s getting collection", err.Error())
	}

	x := 0
	collectionSimplified := make(map[string]simplifiedItem)
	for paymentID := range collectionBlob["redownload_urls"].(map[string]interface{}) {
		tralbum := collectionBlob["tralbums"].([]interface{})[x].(map[string]interface{})
		fmt.Println(tralbum["sale_item_type"].(string) + fmt.Sprint(int(tralbum["sale_item_id"].(float64))))
		collectionSimplified[paymentID] = simplifiedItem{
			itemID:        int(tralbum["item_id"].(float64)),
			redownloadUrl: collectionBlob["redownload_urls"].(map[string]interface{})[tralbum["sale_item_type"].(string)+fmt.Sprint(int(tralbum["sale_item_id"].(float64)))].(string),
			itemType:      tralbum["tralbum_type"].(string),
		}

		albumID := collectionSimplified[paymentID].itemType + fmt.Sprint(collectionSimplified[paymentID].itemID)
		if settings["downloads"].(map[string]interface{})[albumID] == nil {

			downloadBlob, err := scrapeDownload(collectionSimplified[paymentID].redownloadUrl, &client)
			if err != nil {
				log.Fatalf("Error %s getting download url", err.Error())
			}
			album := downloadBlob["digital_items"].([]interface{})[0].(map[string]interface{})
			link := album["downloads"].(map[string]interface{})[settings["format"].(string)].(map[string]interface{})["url"]
			fmt.Println(link)
			fmt.Println(collectionSimplified[paymentID].itemType)
			fmt.Println("downloading ", album["title"].(string))
			file, err := downloadFile(&client, link.(string), makeFileName(album["title"].(string)), collectionSimplified[paymentID].itemType)
			if err != nil {
				log.Fatalf("Error %s downloading file", err.Error())
				break
			}
			if file.Name()[len(file.Name())-3:len(file.Name())] == "zip" {
				zReader, err := zip.OpenReader(file.Name())
				if err != nil {
					log.Fatalf("Error %s reading zip file", err.Error())
				}
				//check if dir exists, if not,make it
				err = os.Mkdir(settings["music_path"].(string)+makeFileName(album["title"].(string)), 0666)
				if err != nil && !os.IsExist(err) {
					log.Fatalf("Error %s making album dir", err.Error())
				}
				for _, f := range zReader.File {
					fReader, err := f.Open()
					if err != nil {
						log.Fatalf("Error %s reading file in archive", err.Error())
					}
					path := fmt.Sprint(settings["music_path"].(string), makeFileName(album["title"].(string)), string(os.PathSeparator), f.Name)
					fmt.Println(path)
					fFinal, err := os.Create(path)
					if err != nil {
						log.Fatalf("Error %s creating file in archive", err.Error())
					}
					_, err = io.Copy(fFinal, fReader)
					if err != nil {
						log.Fatalf("Error %s writing file from archive", err.Error())
					}
					err = fFinal.Close()
					if err != nil {
						log.Fatalf("Error %s closing extracted file", err.Error())
					}
					err = fReader.Close()
					if err != nil {
						log.Fatalf("Error %s closing file reader", err.Error())
					}
				}
				zReader.Close()
				err = file.Close()
				if err != nil {
					log.Fatalf("Error %s closing zip file", err.Error())
				}
				err = os.Remove(file.Name())
				if err != nil {
					log.Fatalf("Error %s removing zip file", err.Error())
				}
			} else {
				file.Close()
			}

			settings["downloads"].(map[string]interface{})[albumID] = true
			saveSettings()
		}
		x++
	}

}
