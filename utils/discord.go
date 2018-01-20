package utils

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

var (
	discord *discordgo.Session
	err     error
)

// will return discord session
func GetDiscordSession() *discordgo.Session {

	// check if bot needs to be initialized
	if discord == nil {
		initDiscordBot()
	}

	return discord
}

// initialize and set up discord bot
func initDiscordBot() {
	fmt.Println("Initializing bot...")

	// Get app configs
	appConfigs := GetAppConfigs()

	// init discord
	discord, err = discordgo.New("Bot dsg" + appConfigs.Path("discord_bot.token").Data().(string))
	PanicCheck(err)

	// Add all event handlers
	addEventHandlers(discord)
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
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// If the message is "ping" reply with "Pong!"
	if m.Content == "ping" {
		s.ChannelMessageSend(m.ChannelID, "Pong!")
	}

	// If the message is "pong" reply with "Ping!"
	if m.Content == "pong" {
		s.ChannelMessageSend(m.ChannelID, "Ping!")
	}
}
