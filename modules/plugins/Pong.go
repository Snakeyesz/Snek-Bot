package plugins

import (
	"github.com/Snakeyesz/snek-bot/utils"
	"github.com/bwmarrin/discordgo"
)

// plugin will simply respond to "ping" with "pong" and vica versa
type Pong struct{}

func (p *Pong) InitPlugin() {}

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

// Main entry point for plugin
func (p *Pong) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {

	// If the message is "ping" reply with "Pong!"
	if command == "ping" {
		utils.SendMessage(msg.ChannelID, "Pong!")
	}

	// If the message is "pong" reply with "Ping!"
	if command == "pong" {
		utils.SendMessage(msg.ChannelID, "Ping!")
	}
}

func (p *Pong) ActionOnReactionAdd(reaction *discordgo.MessageReactionAdd) {

}
