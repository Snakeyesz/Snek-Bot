package cache

import (
	"errors"

	"github.com/Jeffail/gabs"
)

var appConfigs *gabs.Container

func SetAppConfig(config *gabs.Container) {
	appConfigs = config
}

func GetAppConfig() *gabs.Container {

	if appConfigs == nil {
		panic(errors.New("Tried to get discord session before cache#SetSession() was called"))
	}

	return appConfigs
}
