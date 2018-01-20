package utils

import (
	"fmt"

	"github.com/Jeffail/gabs"
)

// Holds all needed app configuration including api tokens, usernames, password, ect...
var appConfigs *gabs.Container

// Will return the bot app configuration
func GetAppConfigs() *gabs.Container {

	// load app configs from file is needed
	if appConfigs.Data() == nil {

		fmt.Println("Loading app Config: ")
		loadAppConfiguration()
		fmt.Println(appConfigs.StringIndent("", " "))
	}

	return appConfigs
}

// Loads all application configuration from config file
func loadAppConfiguration() {
	data, err := gabs.ParseJSONFile("config.json")

	if err != nil {
		panic(err.Error())
	}

	appConfigs = data
}
