package youtube

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/parnurzeal/gorequest"
)

type stream map[string]string

type Youtube struct {
	streamList []stream
	videoID    string
	videoInfo  string
	videoURL   string
}

// loadYoutubeVideoInfo will attempt to retrive load youtube video information based on given url or text
func (y *Youtube) LoadYoutubeVideoInfo(userInput string) (string, string, error) {

	// check if the user is trying to pass a url, if not then search youtube for matching video and return that
	if len(userInput) < 4 || userInput[:4] != "http" {
		// link, err := y.searchYoutube(userInput)
		// if err != nil {
		// 	return "", nil, err
		// }

		// y.videoURL = link

	} else if validateYoutubeUrl(userInput) == false {

		return "", "", errors.New("youtube.invalid-url")
	}

	// set video url
	y.videoURL = userInput

	// set video id
	err := y.setYoutubeID(userInput)
	if err != nil {
		return "", "", err
	}

	// get the video info from youtube
	err = y.retrieveYoutubeInfo()
	if err != nil {
		return "", "", err
	}

	err = y.setStreamList()
	if err != nil {
		return "", "", err
	}

	targetStream := y.streamList[0]
	downloadURL := targetStream["url"] + "&signature=" + targetStream["sig"]
	return downloadURL, targetStream["title"], nil
}

// setYoutubeID takes a url, validates that "url" is a youtube url and then parses out and sets the video id
func (y *Youtube) setYoutubeID(url string) error {
	if !validateYoutubeUrl(url) {
		return errors.New("Invalid video UrL")
	}

	if strings.Contains(url, "youtu") || strings.ContainsAny(url, "\"?&/<%=") {
		reList := []*regexp.Regexp{
			regexp.MustCompile(`(?:v|embed|watch\?v)(?:=|/)([^"&?/=%]{11})`),
			regexp.MustCompile(`(?:=|/)([^"&?/=%]{11})`),
			regexp.MustCompile(`([^"&?/=%]{11})`),
		}
		for _, re := range reList {
			if isMatch := re.MatchString(url); isMatch {
				subs := re.FindStringSubmatch(url)
				y.videoID = subs[1]
				break
			}
		}
	}

	if strings.ContainsAny(y.videoID, "?&/<%=") {
		return errors.New("invalid characters in video id")
	}
	if len(y.videoID) < 10 {
		return errors.New("the video id must be at least 10 characters long")
	}
	return nil
}

// setStreamList uses stream info to parse through and set streamlist
func (y *Youtube) setStreamList() error {
	response, err := url.ParseQuery(y.videoInfo)
	if err != nil {
		return err
	}

	// check if no reponse was given
	status, ok := response["status"]
	if !ok {
		err = fmt.Errorf("no response status found in the server's response")
		return err
	}
	// check if a fail was explicitly given and log the reason if one was given
	if status[0] != "ok" {
		reason, ok := response["reason"]
		if ok {
			err = fmt.Errorf("'fail' response status found in the server's response, reason: '%s'", reason[0])
		} else {
			err = errors.New(fmt.Sprint("'fail' response status found in the server's response, no reason given"))
		}
		return err
	}

	// check if response was not a success but not specifically a fail
	if status[0] != "ok" {
		err = fmt.Errorf("non-success response status found in the server's response (status: '%s')", status)
		return err
	}

	// read the streams map
	streamMap, ok := response["url_encoded_fmt_stream_map"]
	if !ok {
		err = errors.New(fmt.Sprint("no stream map found in the server's response"))
		return err
	}

	// read each stream
	for streamPos, streamRaw := range strings.Split(streamMap[0], ",") {
		streamQry, err := url.ParseQuery(streamRaw)
		if err != nil {
			log.Println(fmt.Sprintf("An error occured while decoding one of the video's stream's information: stream %d: %s\n", streamPos, err))
			continue
		}

		stream := stream{
			"quality": streamQry["quality"][0],
			"type":    streamQry["type"][0],
			"url":     streamQry["url"][0],
			"sig":     "",
			"title":   response["title"][0],
			"author":  response["author"][0],
		}
		if _, exist := streamQry["sig"]; exist {
			stream["sig"] = streamQry["sig"][0]
		}

		y.streamList = append(y.streamList, stream)
	}
	return nil
}

// retrives youtube info from youtube.com/get_video_info based on video id
func (y *Youtube) retrieveYoutubeInfo() error {
	url := "http://youtube.com/get_video_info?video_id=" + y.videoID
	_, body, err := gorequest.New().Get(url).End()
	if err != nil {
		return err[0]
	}
	y.videoInfo = body
	return nil
}

// validateYoutubeUrl will validate the passed url passed is a valid youtube url
//  based on: https://github.com/frozzare/go-youtube-url/blob/master/youtube_url.go
func validateYoutubeUrl(url string) bool {
	m, _ := regexp.MatchString(`((?:http://)?)(?:www\.)?(?:(youtube\.com/(\/watch\?(?:\=.*v=((\w|-){11}))|.+))|(youtu.be\/\w{11}))`, url)
	return m
}
