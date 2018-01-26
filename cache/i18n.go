package cache

import (
	"errors"
	"sync"

	"github.com/Jeffail/gabs"
)

var (
	translations      *gabs.Container
	translationsMutex sync.RWMutex
)

func Seti18nTranslations(t *gabs.Container) {
	translationsMutex.Lock()
	defer translationsMutex.Unlock()

	translations = t
}

func Geti18nTranslations() *gabs.Container {
	translationsMutex.RLock()
	defer translationsMutex.RUnlock()

	if translations == nil {
		panic(errors.New("I18n component was not loaded before get attempt"))
	}

	return translations
}
