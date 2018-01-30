package utils

import (
	"errors"

	"github.com/bwmarrin/discordgo"
)

// JoinUserVoiceChat bot joins the users voice chat if it can find one, returns an error if it can not
func JoinUserVoiceChat(session *discordgo.Session, msg *discordgo.Message) (*discordgo.VoiceConnection, error) {

	// Find the guild based on the channel the message come from
	channel, err := session.State.Channel(msg.ChannelID)
	guild, err := session.State.Guild(channel.GuildID)
	if err != nil {
		return nil, err
	}

	// Look for the message sender in that guild's current voice states.
	for _, vs := range guild.VoiceStates {
		if vs.UserID == msg.Author.ID {

			// join the voice channel and return a possible error
			vc, err := session.ChannelVoiceJoin(guild.ID, vs.ChannelID, false, true)
			return vc, err
		}
	}

	return nil, errors.New("bot.voice.no-target-voice-channel")
}
