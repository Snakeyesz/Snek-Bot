package plugins

import (
	"github.com/Snakeyesz/snek-bot/utils"
	"github.com/bwmarrin/discordgo"
)

// Plugin responds to command by displaying a image retrieved from http://random.cat/meow
type Cat struct{}

func (c *Cat) InitPlugin() {}

// Will validate if the pass command entered is used for this plugin
func (c *Cat) ValidateCommand(command string) bool {
	validCommands := []string{"meow", "cat", "randomcat"}

	for _, v := range validCommands {
		if v == command {
			return true
		}
	}

	return false
}

// Main Entry point for the plugin
func (c *Cat) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {

	// attempt to get cat picture and display if response is valid
	json, err := utils.GetJsonFromUrl("http://random.cat/meow")
	if err != nil {

		utils.SendMessage(
			msg.ChannelID,
			"): something went retrieving the cat pic. sorry",
		)
	} else {

		utils.SendMessage(
			msg.ChannelID,
			"MEOW! :smiley_cat:\n"+json.Path("file").Data().(string),
		)
	}
}
