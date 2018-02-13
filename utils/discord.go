package utils

import (
	"errors"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/bwmarrin/discordgo"
)

// JoinUserVoiceChat bot joins the users voice chat if it can find one, returns an error if it can not
func JoinUserVoiceChat(msg *discordgo.Message) (*discordgo.VoiceConnection, error) {
	session := cache.GetDiscordSession()

	// Find the guild based on the channel the message come from
	channel, err := session.State.Channel(msg.ChannelID)
	guild, err := session.State.Guild(channel.GuildID)
	if err != nil {
		return nil, err
	}

	// Look for the message sender in that guild's current voice states.
	for _, voiceState := range guild.VoiceStates {
		if voiceState.UserID == msg.Author.ID {

			// check if we already have a voice connection in the given guild
			if voiceConnection, ok := session.VoiceConnections[channel.GuildID]; ok {

				// if connection is in the same channel, no more needs to be done. return the voice connection
				if voiceConnection.ChannelID == voiceState.ChannelID {
					return voiceConnection, nil
				}

				// reaching this means we're changing from one channel to another in the same guild
				//   must disconnect before reconnecting to new channel
				voiceConnection.Disconnect()
			}

			// join the voice channel and return a possible error
			voiceConnection, err := session.ChannelVoiceJoin(guild.ID, voiceState.ChannelID, false, true)
			if err == nil {

				return voiceConnection, nil
			} else {

				return nil, errors.New("bot.voice.cant-join-voice")
			}
		}
	}

	return nil, errors.New("bot.voice.no-target-voice-channel")
}

// SendMessage alerts user of custom error if any exists
func SendMessage(channelID string, message string) {
	translations := cache.Geti18nTranslations()

	cache.GetDiscordSession().ChannelTyping(channelID)

	// check if the error code has a user translation
	if translations.ExistsP(message) {
		message = translations.Path(message).Data().(string)
	}

	// output translation to user
	cache.GetDiscordSession().ChannelMessageSend(channelID, message)
}
