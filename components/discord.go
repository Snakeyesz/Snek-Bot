package components

import (
	"fmt"
	"strings"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/Snakeyesz/snek-bot/modules"
	"github.com/Snakeyesz/snek-bot/utils"
	"github.com/bwmarrin/discordgo"
)

var (
	prefix string = "!"
)

// initialize and set up discord bot
func InitDiscordBot() {
	fmt.Println("Initializing discord bot...")

	// Get app configs
	appConfigs := cache.GetAppConfig()

	// init discord
	discord, err := discordgo.New("Bot " + appConfigs.Path("discord_bot.token").Data().(string))
	utils.PanicCheck(err)

	// Add all event handlers
	addEventHandlers(discord)

	cache.SetDiscordSession(discord)
}

// Adds event handlers to discord bot
func addEventHandlers(discord *discordgo.Session) {
	// add handlers
	discord.AddHandler(messageCreate)

}

/**********************************
		   Event Handlers
**********************************/

// Called everytime a message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, msg *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	if msg.Author.ID == s.State.User.ID {
		return
	}

	// If the text has the bots prefix, call plugins
	if strings.HasPrefix(msg.Content, prefix) {
		trimmedMessage := strings.TrimPrefix(msg.Content, prefix)

		// bot command
		command := strings.SplitN(trimmedMessage, " ", 2)[0]

		// check for user input passed the command
		userText := ""
		if len(strings.SplitN(trimmedMessage, " ", 2)) > 1 {
			userText = strings.SplitN(trimmedMessage, " ", 2)[1]
		}

		modules.CallBotPlugin(command, userText, msg.Message)

		fmt.Println(trimmedMessage)
		fmt.Println(command)
		fmt.Println(userText)

	}
}