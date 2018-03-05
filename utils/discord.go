package utils

import (
	"errors"
	"io"

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

// SendMessage sends a message to the given channel. will translate message if an i18n translation exists
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

// SendEmbed sends a message to the given channel using an embeded form
func SendEmbed(channelID string, embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	cache.GetDiscordSession().ChannelTyping(channelID)

	// output translation to user
	embededMessage, err := cache.GetDiscordSession().ChannelMessageSendEmbed(channelID, TruncateEmbed(embed))
	return embededMessage, err
}

// EditEmbed edits the embed message with the new embed
func EditEmbed(channelID string, messageID string, embed *discordgo.MessageEmbed) (message *discordgo.Message, err error) {
	message, err = cache.GetDiscordSession().ChannelMessageEditEmbed(channelID, messageID, TruncateEmbed(embed))
	if err != nil {
		return nil, err
	} else {
		return message, err
	}
}

// s alerts user of custom error if any exists
func SendFile(channelID string, filename string, reader io.Reader, message string) (*discordgo.Message, error) {
	// cache.GetDiscordSession().ChannelTyping(channelID)

	if message != "" {

		return cache.GetDiscordSession().ChannelFileSendWithMessage(channelID, message, filename, reader)
	} else {

		return cache.GetDiscordSession().ChannelFileSend(channelID, filename, reader)
	}
}

// GetGuildFromMessage
func GetGuildFromMessage(msg *discordgo.Message) (*discordgo.Guild, error) {

	channel, err := cache.GetDiscordSession().State.Channel(msg.ChannelID)
	if err != nil {
		return nil, err
	}

	guild, err := cache.GetDiscordSession().State.Guild(channel.GuildID)
	if err != nil {
		return nil, err
	}

	return guild, nil
}

// Applies Embed Limits to the given Embed
// Source: https://discordapp.com/developers/docs/resources/channel#embed-limits
func TruncateEmbed(embed *discordgo.MessageEmbed) (result *discordgo.MessageEmbed) {
	if embed == nil || (&discordgo.MessageEmbed{}) == embed {
		return nil
	}
	if embed.Title != "" && len(embed.Title) > 256 {
		embed.Title = embed.Title[0:255] + "…"
	}
	if len(embed.Description) > 2048 {
		embed.Description = embed.Description[0:2047] + "…"
	}
	if embed.Footer != nil && len(embed.Footer.Text) > 2048 {
		embed.Footer.Text = embed.Footer.Text[0:2047] + "…"
	}
	if embed.Author != nil && len(embed.Author.Name) > 256 {
		embed.Author.Name = embed.Author.Name[0:255] + "…"
	}
	newFields := make([]*discordgo.MessageEmbedField, 0)
	for _, field := range embed.Fields {
		if field.Value == "" {
			continue
		}
		if len(field.Name) > 256 {
			field.Name = field.Name[0:255] + "…"
		}
		// TODO: better cutoff (at commas and stuff)
		if len(field.Value) > 1024 {
			field.Value = field.Value[0:1023] + "…"
		}
		newFields = append(newFields, field)
		if len(newFields) >= 25 {
			break
		}
	}
	embed.Fields = newFields

	if CalculateFullEmbedLength(embed) > 6000 {
		if embed.Footer != nil {
			embed.Footer.Text = ""
		}
		if CalculateFullEmbedLength(embed) > 6000 {
			if embed.Author != nil {
				embed.Author.Name = ""
			}
			if CalculateFullEmbedLength(embed) > 6000 {
				embed.Fields = []*discordgo.MessageEmbedField{{}}
			}
		}
	}

	result = embed
	return result
}

func CalculateFullEmbedLength(embed *discordgo.MessageEmbed) (count int) {
	count += len(embed.Title)
	count += len(embed.Description)
	if embed.Footer != nil {
		count += len(embed.Footer.Text)
	}
	if embed.Author != nil {
		count += len(embed.Author.Name)
	}
	for _, field := range embed.Fields {
		count += len(field.Name)
		count += len(field.Value)
	}
	return count
}
