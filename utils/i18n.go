package utils

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/Snakeyesz/snek-bot/cache"
)

func Geti18nText(id string) string {
	translations := cache.Geti18nTranslations()

	// check if the requested key has a corresponding value, if not return key
	if !translations.ExistsP(id) {
		return id
	}

	item := translations.Path(id)

	// If this is an object return __
	if strings.Contains(item.String(), "{") {
		item = item.Path("__")
	}

	// If this is an array return a random item
	if strings.Contains(item.String(), "[") {
		arr := item.Data().([]interface{})
		return arr[rand.Intn(len(arr))].(string)
	}

	return item.Data().(string)
}

func Geti18nTextF(id string, replacements ...interface{}) string {
	return fmt.Sprintf(GetText(id), replacements...)
}
