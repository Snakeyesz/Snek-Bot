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

	// init discord
	discord, err := discordgo.New("Bot " + cache.GetAppConfig().Path("discord_bot.token").Data().(string))
	utils.PanicCheck(err)
	cache.SetDiscordSession(discord)

	// add event handlers
	addEventHandlers(discord)
}

// Adds event handlers to discord bot
func addEventHandlers(discord *discordgo.Session) {

	// add handlers
	discord.AddHandler(messageCreate)
	discord.AddHandler(botOnReactionAdd)
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

		// pass command to plugin handler
		modules.CallBotPlugin(command, userText, msg.Message)
	}
}

// Called everytime a reaction is added to any message
func botOnReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.UserID == s.State.User.ID {
		return
	}

	modules.CallBotPluginOnReactionAdd(r)
}
