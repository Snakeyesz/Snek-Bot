package modules

/**
 * Entry point for plugins.
 * This will check to see if a command or action has occurred in which a plugin needs to act upon or react to
 */

import (
	"strings"

	"github.com/Snakeyesz/snek-bot/modules/plugins/biasgame"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/Snakeyesz/snek-bot/modules/plugins"
	"github.com/bwmarrin/discordgo"
)

// Basic interface for plugins
type Plugin interface {
	// custom init for the plugins
	//   golang init fires to soon in some cases
	InitPlugin()

	// Simple and efficient check if the passed command is valid
	ValidateCommand(command string) bool

	// Entry point for plugig
	// Action should also validate commands sent to it first to avoid running incorrectly
	Action(
		command string,
		content string,
		msg *discordgo.Message,
		session *discordgo.Session,
	)

	ActionOnReactionAdd(reaction *discordgo.MessageReactionAdd)
}

// List of active plugins
var pluginList []Plugin

func InitPlugins() {
	pluginList = []Plugin{
		&plugins.Cat{},
		&plugins.Pong{},
		&plugins.Music{},
		&biasgame.BiasGame{},
	}

	for _, plugin := range pluginList {
		go plugin.InitPlugin()
	}
}

// command - The command that triggered this execution
// content - The content without command
// msg     - The message object
func CallBotPlugin(command string, content string, msg *discordgo.Message) {
	// Convert to command to lowercase
	command = strings.ToLower(command)

	// Run plugins for the given command
	for _, plugin := range pluginList {

		if plugin.ValidateCommand(command) {
			plugin.Action(command, content, msg, cache.GetDiscordSession())
		}
	}
}

func CallBotPluginOnReactionAdd(reaction *discordgo.MessageReactionAdd) {

	/// Run plugins for the given command
	for _, plugin := range pluginList {

		plugin.ActionOnReactionAdd(reaction)
	}
}
