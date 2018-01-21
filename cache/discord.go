package cache

import (
	"errors"
	"sync"

	"github.com/bwmarrin/discordgo"
)

var (
	session      *discordgo.Session
	sessionMutex sync.RWMutex
)

func SetDiscordSession(s *discordgo.Session) {
	sessionMutex.Lock()
	session = s
	sessionMutex.Unlock()
}

func GetDiscordSession() *discordgo.Session {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()

	if session == nil {
		panic(errors.New("Tried to get discord session before cache#SetSession() was called"))
	}

	return session
}
