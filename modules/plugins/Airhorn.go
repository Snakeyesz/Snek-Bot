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
//  no validation is done on if the user initaiting is in a current voice channel, may do this later but this plugin isn't important
type Airhorn struct{}

// buffer for sound file
var soundFileBuffer = make([][]byte, 0)

func init() {
	loadSound()
}

// will validate if the pass command is used for this plugin
func (a *Airhorn) ValidateCommand(command string) bool {
	validCommands := []string{"airhorn", "horn"}

	for _, v := range validCommands {
		if v == command {
			return true
		}
	}

	return false
}

// Main Entry point for the plugin
func (a *Airhorn) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {
	if a.ValidateCommand(command) == false {
		return
	}

	// Find the channel that the message came from.
	channel, err := session.State.Channel(msg.ChannelID)
	utils.PanicCheck(err)

	// Find the guild for that channel.
	guild, err := session.State.Guild(channel.GuildID)
	utils.PanicCheck(err)

	// Look for the message sender in that guild's current voice states.
	for _, vs := range guild.VoiceStates {
		if vs.UserID == msg.Author.ID {
			playSoundFile(session, guild.ID, vs.ChannelID)
			return
		}
	}
}

// loadSound attempts to load an encoded sound file from disk.
func loadSound() {

	file, err := os.Open("modules/plugins/airhorn/airhorn.dca")
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
		soundFileBuffer = append(soundFileBuffer, InBuf)
	}
}

// playSoundFile plays the current buffer to the provided channel.
func playSoundFile(s *discordgo.Session, guildID, channelID string) {

	// Join the provided voice channel.
	vc, err := s.ChannelVoiceJoin(guildID, channelID, false, true)
	utils.PanicCheck(err)

	// Sleep for a specified amount of time before playing the sound
	time.Sleep(250 * time.Millisecond)

	// Start speaking.
	vc.Speaking(true)

	// Send the buffer data.
	for _, buff := range soundFileBuffer {
		vc.OpusSend <- buff
	}

	// Stop speaking
	vc.Speaking(false)

	// Sleep for a specificed amount of time before ending.
	time.Sleep(500 * time.Millisecond)

	// Disconnect from the provided voice channel.
	vc.Disconnect()
}
