package plugins

import (
	"encoding/binary"
	"io"
	"os"
	"time"

	"github.com/Snakeyesz/snek-bot/utils"
	"github.com/bwmarrin/discordgo"
)

// Plugin joins voice chat of the user that initiated it.
type Music struct{}

// buffer for sound file
var buffer = make([][]byte, 0)

func init() {
	loadSound()
}

// will validate if the pass command is used for this plugin
func (p *Music) ValidateCommand(command string) bool {
	validCommands := []string{"play", "stop"}

	for _, v := range validCommands {
		if v == command {
			return true
		}
	}

	return false
}

// Main Entry point for the plugin
func (p *Music) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if p.ValidateCommand(command) == false {
		return
	}

	// If the message is "ping" reply with "Pong!"
	if command == "play" {

		// Find the channel that the message came from.
		channel, err := session.State.Channel(msg.ChannelID)
		utils.PanicCheck(err)

		// Find the guild for that channel.
		guild, err := session.State.Guild(channel.GuildID)
		utils.PanicCheck(err)

		// Look for the message sender in that guild's current voice states.
		for _, vs := range guild.VoiceStates {
			if vs.UserID == msg.Author.ID {
				playSound(session, guild.ID, vs.ChannelID)
				return
			}
		}
	}

	// If the message is "pong" reply with "Ping!"
	if command == "stop" {
		// session.ChannelMessageSend(msg.ChannelID, "Ping!")
	}
}

// loadSound attempts to load an encoded sound file from disk.
func loadSound() {

	file, err := os.Open("modules/plugins/music/airhorn.dca")
	utils.PanicCheck(err)
	var opuslen int16

	for {
		// Read opus frame length from dca file.
		err = binary.Read(file, binary.LittleEndian, &opuslen)

		// If this is the end of the file, just return.
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			file.Close()
			return
		}
		utils.PanicCheck(err)

		// Read encoded pcm from dca file.
		InBuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &InBuf)
		utils.PanicCheck(err)

		// Append encoded pcm data to the buffer.
		buffer = append(buffer, InBuf)
	}
}

// playSound plays the current buffer to the provided channel.
func playSound(s *discordgo.Session, guildID, channelID string) {

	// Join the provided voice channel.
	vc, err := s.ChannelVoiceJoin(guildID, channelID, false, true)
	utils.PanicCheck(err)

	// Sleep for a specified amount of time before playing the sound
	time.Sleep(250 * time.Millisecond)

	// Start speaking.
	vc.Speaking(true)

	// Send the buffer data.
	for _, buff := range buffer {
		vc.OpusSend <- buff
	}

	// Stop speaking
	vc.Speaking(false)

	// Sleep for a specificed amount of time before ending.
	time.Sleep(250 * time.Millisecond)

	// Disconnect from the provided voice channel.
	vc.Disconnect()
}
