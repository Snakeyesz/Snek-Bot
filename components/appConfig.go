package components

import (
	"fmt"

	"github.com/Jeffail/gabs"
	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/Snakeyesz/snek-bot/utils"
)

// Load app configuration from config file
func LoadAppConfig() {
	fmt.Println("Loading app Config...")

	data, err := gabs.ParseJSONFile("config.json")
	utils.PanicCheck(err)

	cache.SetAppConfig(data)
}
