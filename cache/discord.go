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
	defer sessionMutex.Unlock()

	session = s
}

func GetDiscordSession() *discordgo.Session {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()

	if session == nil {
		panic(errors.New("Discord component was not loaded before get attempt"))
	}

	return session
}
