package plugins

import (
	"github.com/Snakeyesz/snek-bot/utils"
	"github.com/bwmarrin/discordgo"
)

type Cat struct{}

// will validate if the pass command is used for this plugin
func (c *Cat) ValidateCommand(command string) bool {
	validCommands := []string{"meow", "cat", "randomcat"}

	for _, v := range validCommands {
		if v == command {
			return true
		}
	}

	return false
}

func (c *Cat) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if c.ValidateCommand(command) == false {
		return
	}

	session.ChannelTyping(msg.ChannelID)

	json := utils.GetJsonFromUrl("http://random.cat/meow")
	session.ChannelMessageSend(
		msg.ChannelID,
		"MEOW! :smiley_cat:\n"+json.Path("file").Data().(string),
	)
}
