package components

import (
	"fmt"

	"github.com/Jeffail/gabs"
	"github.com/Snakeyesz/snek-bot/cache"
)

// Load i18n cache from json file
func Loadi18nTranslations() {
	fmt.Println("Loading i18n file...")

	json, err := gabs.ParseJSONFile("assets/i18n.json")
	utils.PanicCheck(err)

	cache.Seti18nTranslations(data)
}
