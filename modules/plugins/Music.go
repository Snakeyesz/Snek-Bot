package plugins

import (
	"fmt"

	"github.com/Snakeyesz/snek-bot/modules/plugins/voice"

	"github.com/bwmarrin/discordgo"
)

// Plugin joins voice chat of the user that initiated it and plays music based on the passed link
type Music struct{}

func (p *Music) InitPlugin() {}

// will validate if the pass command is used for this plugin
func (p *Music) ValidateCommand(command string) bool {
	validCommands := []string{
		"play",
		"stop",
		"skip",
		"pause",
		"unpause",
		"repeat",
		"shuffle",
	}

	for _, v := range validCommands {
		if v == command {
			return true
		}
	}

	return false
}

// Main Entry point for the plugin
func (p *Music) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {

	channel, err := session.State.Channel(msg.ChannelID)
	if err != nil {
		fmt.Println(err)
		return
	}

	// get voice instance for the users guild
	voiceInstance := voice.GetOrMakeVoiceInstance(channel.GuildID, msg)

	switch command {
	case "play":

		// if content length is nothing, assume they're playing from a pause
		if len(content) == 0 {

			voiceInstance.TogglePauseSong(false)

		} else {
			voiceInstance.PlaySongByUrl(content, msg)
		}
	case "stop":
		voiceInstance.StopMusic()
	case "skip":
		voiceInstance.SkipSong()
	case "pause":
		voiceInstance.TogglePauseSong(true)
	case "unpause":
		voiceInstance.TogglePauseSong(false)
	case "repeat":
		voiceInstance.RepeatSong(msg)
		// case "shuffle":

	}
}

func (m *Music) ActionOnReactionAdd(reaction *discordgo.MessageReactionAdd) {

}
