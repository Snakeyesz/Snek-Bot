package cache

import (
	"errors"
	"sync"

	"github.com/Jeffail/gabs"
)

var (
	appConfigs      *gabs.Container
	appConfigsMutex sync.RWMutex
)

func SetAppConfig(config *gabs.Container) {
	appConfigsMutex.Lock()
	defer appConfigsMutex.Unlock()

	appConfigs = config
}

func GetAppConfig() *gabs.Container {
	appConfigsMutex.RLock()
	defer appConfigsMutex.RUnlock()

	if appConfigs == nil {
		panic(errors.New("AppConfig component was not loaded before get attempt"))
	}

	return appConfigs
}
