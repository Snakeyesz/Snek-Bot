package voice

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"

	"github.com/Snakeyesz/snek-bot/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/jeffreymkabot/ytdl"
	"github.com/oleiade/lane"
	"layeh.com/gopus"
)

const (
	audioChannels int = 2                               // 1 for mono, 2 for stereo
	frameRate     int = 48000                           // audio sampling rate
	frameSize     int = 960                             // uint16 size of each audio frame
	maxBytes      int = (frameSize * audioChannels) * 2 // max size of opus data
	audioBitrate  int = 96000                           // audio bit rate
)

var (
	// channel to send audio packets to discord
	recv chan *discordgo.Packet

	// Holds the voiceInstances for all servers the bot is on
	// serverid: voiceInstance{}
	voiceInstances = map[string]*voiceInstance{}
)

// VoiceInstance is created for each connected server
type voiceInstance struct {
	queue           *lane.Queue
	voiceConnection *discordgo.VoiceConnection
	pcmChannel      chan []int16
	guildID         string
	skip            bool
	stop            bool
	repeat          bool
	pause           bool
	trackPlaying    bool

	// the text channel the music was initiated from.
	// will be used to send error messages or update messages.
	textChannelID string
}

//////////////////////
// PUBLIC FUNCTIONS //
//////////////////////

// GetVoiceInstance will return the voice instance or create one if doesn't exist for the given guild
func GetVoiceInstance(guildID string, msg *discordgo.Message) *voiceInstance {

	// if the voiceinstance exists, return it
	if vi, ok := voiceInstances[guildID]; ok {

		return vi
	} else {

		vi := new(voiceInstance)

		// set up voiceInstance and add it to voiceInstances
		vi.queue = lane.NewQueue()
		vi.textChannelID = msg.ChannelID
		vi.guildID = guildID
		voiceInstances[guildID] = vi

		return vi
	}
}

// PlaySongByUrl will get the songs details, join the users channel, add the song to the queue and start processing the queue
func (vi *voiceInstance) PlaySongByUrl(url string, msg *discordgo.Message) {

	// get the actual download link from the url
	downloadLink, err := getDownloadUrl(url)
	if err != nil {
		utils.SendMessage(msg.ChannelID, "music.invalid-url")
		return
	}

	// if not already in a channel, attempt to join the users channel
	voiceConnection, err := utils.JoinUserVoiceChat(msg)
	if err != nil {
		utils.SendMessage(msg.ChannelID, err.Error())
		return
	} else {
		fmt.Println("voice connection successful")
		vi.voiceConnection = voiceConnection
	}

	// add song to queue
	vi.queue.Enqueue(downloadLink)
	fmt.Println(vi.queue.Size())
	if vi.queue.Size() == 1 {
		// alert user song is playing

	} else {

		// alert user song was queued
		utils.SendMessage(msg.ChannelID, "music.song-queued")
	}

	vi.processQueue()
}

// togglePauseSong will pause or unpause song playing
func (vi *voiceInstance) TogglePauseSong(toggle bool) {
	vi.pause = toggle
}

// stopSong will stop the current song, voiceinstance queue, and leave the channel
func (vi *voiceInstance) StopMusic() {
	vi.stop = true
}

// skipSong will skip to the next song in the queue if there is one and will turn off repeat
func (vi *voiceInstance) SkipSong() {
	vi.repeat = false
	vi.skip = true
}

// repeatSong repeat the song continuously
func (vi *voiceInstance) RepeatSong(msg *discordgo.Message) {

	vi.repeat = !vi.repeat

	if vi.repeat == true {

		utils.SendMessage(msg.ChannelID, "music.repeat.start")
	} else {

		utils.SendMessage(msg.ChannelID, "music.repeat.end")
	}
}

///////////////////////
// PRIVATE FUNCTIONS //
///////////////////////

func (vi *voiceInstance) processQueue() {
	fmt.Println("processing queue")

	if vi.trackPlaying == false {
		for {
			vi.skip = false

			link := vi.queue.Head()
			if link == nil || vi.stop == true {
				vi.queue.Dequeue()
				break
			}

			vi.startAudioStreamFromUrl(link.(string))

			if vi.repeat == false {
				vi.queue.Dequeue()
			}
		}

		// No more tracks in queue? Cleanup.
		fmt.Println("Closing connections")

		if vi.stop == true {
			vi.voiceConnection.Disconnect()
		}
	}
}

// startAudioStreamFromUrl will start reading the buffer
func (vi *voiceInstance) startAudioStreamFromUrl(url string) {
	fmt.Println("playing form url")
	vi.trackPlaying = true

	// get the audio data from the given song url
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != 200 {
		// TODO: could not stream audio skip to next song
		return
	}
	defer resp.Body.Close()

	// if the pcm channel is not yet running, start it
	if vi.pcmChannel == nil {
		vi.pcmChannel = make(chan []int16, 1)
		go sendPCM(vi.voiceConnection, vi.pcmChannel)
	}

	// option details can be found https://ffmpeg.org/ffmpeg-all.html
	args := []string{
		"-i", "-",
		"-f", "s16le",
		"-ar", strconv.Itoa(frameRate),
		"-ac", strconv.Itoa(audioChannels),
		"-b:a", strconv.Itoa(audioChannels),
		"pipe:1",
	}

	// use ffmpeg to read and convert the audio to pcm
	ffmpeg := exec.Command("ffmpeg", args...)
	ffmpeg.Stdin = resp.Body
	stdout, err := ffmpeg.StdoutPipe()
	if err != nil {
		fmt.Println("ffmpeg stdoutpipe error:", err)
		return
	}

	err = ffmpeg.Start()
	if err != nil {
		fmt.Println("RunStart Error:", err)
		return
	}

	// audio buffer used during loop below
	audiobuf := make([]int16, frameSize*audioChannels)

	// show and allow the bot to output audio
	err = vi.voiceConnection.Speaking(true)
	if err != nil {
		// alert user bot can't properly output audio and stop music
		utils.SendMessage(vi.textChannelID, "bot.voice.bot-cant-speak")
		vi.stop = true
	}
	defer vi.voiceConnection.Speaking(false)

	for {

		if vi.pause == true {
			fmt.Println("pauseing song")
			continue
		}

		if vi.stop == true || vi.skip == true {
			fmt.Println("stopping song")
			ffmpeg.Process.Kill()
			break
		}

		// read data from ffmpeg stdout
		err = binary.Read(stdout, binary.LittleEndian, &audiobuf)
		if err == io.EOF {
			fmt.Println("EOF")
			ffmpeg.Process.Kill()
			break
		}

		if err == io.ErrUnexpectedEOF {
			fmt.Println("ErrUnexpectedEOF")
			ffmpeg.Process.Kill()
			break
		}
		if err != nil {
			fmt.Println("error reading from ffmpeg stdout :", err)
			ffmpeg.Process.Kill()
			break
		}

		vi.pcmChannel <- audiobuf
	}

	fmt.Println("read loop ended")
	vi.trackPlaying = false
}

// sendPCM will create the pcm channel which sends opus data to discord from the pcm channel which will be loaded with audio
func sendPCM(v *discordgo.VoiceConnection, pcm <-chan []int16) {
	if pcm == nil {
		return
	}

	var err error

	opusEncoder, err := gopus.NewEncoder(frameRate, audioChannels, gopus.Audio)
	opusEncoder.SetBitrate(audioBitrate)

	if err != nil {
		fmt.Println("NewEncoder Error", err)
		return
	}

	for {

		// read pcm from chan, exit if channel is closed.
		recv, ok := <-pcm
		if !ok {
			fmt.Println("PCM Channel closed", nil)
			return
		}

		// try encoding pcm frame with Opus
		opus, err := opusEncoder.Encode(recv, frameSize, maxBytes)
		if err != nil {
			fmt.Println("Encoding Error", err)
			return
		}

		if v.Ready == false || v.OpusSend == nil {
			return
		}
		// send encoded opus data to the sendOpus channel
		v.OpusSend <- opus
	}
}

// getDownloadUrl will get the download link for the users input.This can be a single video, a playlist, a text to search for, or an audio file
func getDownloadUrl(url string) (string, error) {
	fmt.Println(url)

	video, err := ytdl.GetVideoInfo(url)
	if err != nil || video.ID == "" || len(video.Formats) == 0 {
		return "", errors.New("music.no-video-information")
	}

	// download audio at best quality
	formats := video.Formats.Best(ytdl.FormatAudioBitrateKey)[0]
	downloadLink, err := video.GetDownloadURL(formats)
	if err != nil {
		return "", errors.New("music.no-video-information")
	}

	return downloadLink.String(), nil
}
