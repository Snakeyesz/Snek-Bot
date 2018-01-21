package utils

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"

	"time"

	"github.com/Jeffail/gabs"
)

var DEFAULT_UA = "SnekBot (https://github.com/Snakeyesz/Snek-Bot)"

// NetGet executes a GET request to url with SnekBot User agent
func NetGet(url string) []byte {
	return NetGetUA(url, DEFAULT_UA)
}

// NetGetUA performs a GET request with a custom user-agent
func NetGetUA(url string, useragent string) []byte {
	// Allocate client
	client := &http.Client{
		Timeout: time.Duration(15 * time.Second),
	}

	// Prepare request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}

	// Set custom UA
	request.Header.Set("User-Agent", useragent)

	// Do request
	response, err := client.Do(request)
	PanicCheck(err)

	// Only continue if code was 200
	if response.StatusCode != 200 {
		panic(errors.New("Expected status 200; Got " + strconv.Itoa(response.StatusCode)))
	} else {
		// Read body
		defer response.Body.Close()

		buf := bytes.NewBuffer(nil)
		_, err := io.Copy(buf, response.Body)
		PanicCheck(err)

		return buf.Bytes()
	}
}

func NetGetUAWithError(url string, useragent string) ([]byte, error) {
	// Allocate client
	client := &http.Client{
		Timeout: time.Duration(15 * time.Second),
	}

	// Prepare request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []byte{}, err
	}

	// Set custom UA
	request.Header.Set("User-Agent", useragent)

	// Do request
	response, err := client.Do(request)
	if err != nil {
		return []byte{}, err
	}

	// Only continue if code was 200
	if response.StatusCode != 200 {
		return []byte{}, errors.New("expected status 200; got " + strconv.Itoa(response.StatusCode))
	} else {
		// Read body
		defer response.Body.Close()

		buf := bytes.NewBuffer(nil)
		_, err := io.Copy(buf, response.Body)
		if err != nil {
			return []byte{}, err
		}

		return buf.Bytes(), nil
	}
	return []byte{}, errors.New("internal error")
}

func NetGetUAWithErrorAndTransport(url string, useragent string, transport http.Transport) ([]byte, error) {
	// Allocate client
	client := &http.Client{
		Timeout:   time.Duration(15 * time.Second),
		Transport: &transport,
	}

	// Prepare request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []byte{}, err
	}

	// Set custom UA
	request.Header.Set("User-Agent", useragent)

	// Do request
	response, err := client.Do(request)
	if err != nil {
		return []byte{}, err
	}

	// Only continue if code was 200
	if response.StatusCode != 200 {
		return []byte{}, errors.New("expected status 200; got " + strconv.Itoa(response.StatusCode))
	} else {
		// Read body
		defer response.Body.Close()

		buf := bytes.NewBuffer(nil)
		_, err := io.Copy(buf, response.Body)
		if err != nil {
			return []byte{}, err
		}

		return buf.Bytes(), nil
	}
	return []byte{}, errors.New("internal error")
}

func NetGetUAWithErrorAndTimeout(url string, useragent string, timeout time.Duration) ([]byte, error) {
	// Allocate client
	client := &http.Client{
		Timeout: timeout,
	}

	// Prepare request
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []byte{}, err
	}

	// Set custom UA
	request.Header.Set("User-Agent", useragent)

	// Do request
	response, err := client.Do(request)
	if err != nil {
		return []byte{}, err
	}

	// Only continue if code was 200
	if response.StatusCode != 200 {
		if err != nil {
			return []byte{}, errors.New("Expected status 200; Got " + strconv.Itoa(response.StatusCode))
		}
	} else {
		// Read body
		defer response.Body.Close()

		buf := bytes.NewBuffer(nil)
		_, err := io.Copy(buf, response.Body)
		if err != nil {
			return []byte{}, err
		}

		return buf.Bytes(), nil
	}
	return []byte{}, errors.New("Internal Error")
}

func NetPost(url string, data string) []byte {
	return NetPostUA(url, data, DEFAULT_UA)
}

func NetPostUA(url string, data string, useragent string) []byte {
	// Allocate client
	client := &http.Client{
		Timeout: time.Duration(15 * time.Second),
	}

	// Prepare request
	request, err := http.NewRequest("POST", url, bytes.NewBufferString(data))
	if err != nil {
		panic(err)
	}

	request.Header.Set("User-Agent", useragent)
	request.Header.Set("Content-Type", "application/json")

	// Do request
	response, err := client.Do(request)
	PanicCheck(err)

	// Only continue if code was 200
	if response.StatusCode != 200 {
		panic(errors.New("Expected status 200; Got " + strconv.Itoa(response.StatusCode)))
	} else {
		// Read body
		defer response.Body.Close()

		buf := bytes.NewBuffer(nil)
		_, err := io.Copy(buf, response.Body)
		PanicCheck(err)

		return buf.Bytes()
	}
}

func NetPostUAWithError(url string, data string, useragent string) (result []byte, err error) {
	// Allocate client
	client := &http.Client{
		Timeout: time.Duration(15 * time.Second),
	}

	// Prepare request
	request, err := http.NewRequest("POST", url, bytes.NewBufferString(data))
	if err != nil {
		return result, err
	}

	request.Header.Set("User-Agent", useragent)
	request.Header.Set("Content-Type", "application/json")

	// Do request
	response, err := client.Do(request)
	if err != nil {
		return result, err
	}

	// Only continue if code was 200
	if response.StatusCode != 200 {
		return result, errors.New("Expected status 200; Got " + strconv.Itoa(response.StatusCode))
	} else {
		// Read body
		defer response.Body.Close()

		buf := bytes.NewBuffer(nil)
		_, err := io.Copy(buf, response.Body)
		if err != nil {
			return result, err
		}

		return buf.Bytes(), nil
	}
}

// GetJSON sends a GET request to $url, parses it and returns the JSON
func GetJsonFromUrl(url string) *gabs.Container {
	// Parse json
	json, err := gabs.ParseJSON(NetGet(url))
	PanicCheck(err)

	return json
}
