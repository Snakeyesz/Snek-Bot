package plugins

import (
	"github.com/bwmarrin/discordgo"
)

type Pong struct{}

// will validate if the pass command is used for this plugin
func (p *Pong) ValidateCommand(command string) bool {
	validCommands := []string{"ping", "pong"}

	for _, v := range validCommands {
		if v == command {
			return true
		}
	}

	return false
}

func (p *Pong) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {

	session.ChannelTyping(msg.ChannelID)

	// If the message is "ping" reply with "Pong!"
	if command == "ping" {
		session.ChannelMessageSend(msg.ChannelID, "Pong!")
	}

	// If the message is "pong" reply with "Ping!"
	if command == "pong" {
		session.ChannelMessageSend(msg.ChannelID, "Ping!")
	}
}
