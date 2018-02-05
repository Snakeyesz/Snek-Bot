package plugins

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"sync"

	"github.com/Snakeyesz/snek-bot/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/layeh/gopus"
	"github.com/oleiade/lane"
	"github.com/otium/ytdl"
)

// Plugin joins voice chat of the user that initiated it and plays music based on the passed link
type Music struct{}

const (
	channels  int = 2                   // 1 for mono, 2 for stereo
	frameRate int = 48000               // audio sampling rate
	frameSize int = 960                 // uint16 size of each audio frame
	maxBytes  int = (frameSize * 2) * 2 // max size of opus data
)

var (
	run     *exec.Cmd
	sendpcm bool
	recv    chan *discordgo.Packet
	mu      sync.Mutex

	// serverid: voiceInstance{}
	voiceInstances = map[string]*voiceInstance{}
)

// VoiceInstance is created for each connected server
type voiceInstance struct {
	discord         *discordgo.Session
	queue           *lane.Queue
	voiceConnection *discordgo.VoiceConnection
	pcmChannel      chan []int16
	serverID        string
	skip            bool
	stop            bool
	pause           bool
	trackPlaying    bool
}

// will validate if the pass command is used for this plugin
func (p *Music) ValidateCommand(command string) bool {
	validCommands := []string{"play", "stop", "skip", "pause"}

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

	channel, err := session.State.Channel(msg.ChannelID)
	if err != nil {
		fmt.Println(err)
		return
	}

	if command == "play" && len(content) > 0 {
		fmt.Println("playing input")

		downloadLink, err := getDownloadUrl(content)
		if err != nil {
			utils.SendMessage(msg.ChannelID, "music.invalid-url")
			return
		}

		// join the users channel
		voiceConnection, err := utils.JoinUserVoiceChat(msg)
		if err != nil {
			utils.SendMessage(msg.ChannelID, err.Error())
			return
		}

		// if a voiceinstances already exists for the given server, add the song to queue.
		//  Otherwise create voice instance
		if vc, ok := voiceInstances[channel.GuildID]; ok {

			vc.queue.Enqueue(downloadLink)
			utils.SendMessage(msg.ChannelID, "music.song-queued")
		} else {

			go createVoiceInstance(downloadLink, channel.GuildID, voiceConnection)
		}
	}

	if command == "play" {
		fmt.Println("continue current song")
		togglePauseSong(channel.GuildID, false)
	}

	if command == "stop" {
		stopSong(channel.GuildID)
	}

	if command == "skip" {
		skipSong(channel.GuildID)
	}

	if command == "pause" {
		togglePauseSong(channel.GuildID, true)
	}

	if command == "repeat" {

	}

	if command == "shuffle" {

	}
}

// getDownloadUrl will get the download link for the users input.This can be a single video, a playlist, a text to search for, or an audio file
func getDownloadUrl(userUrl string) (string, error) {

	video, err := ytdl.GetVideoInfo(userUrl)
	if err != nil {
		return "", err
	}

	// get the video data using an accepted format
	for _, format := range video.Formats {

		if format.AudioEncoding == "opus" || format.AudioEncoding == "aac" || format.AudioEncoding == "vorbis" {

			downloadLink, err := video.GetDownloadURL(format)
			if err != nil {
				fmt.Println(err)
				return "", err
			}

			return downloadLink.String(), nil
		}
	}
	return "", errors.New("music.no-video-information")
}

func stopSong(guildId string) {
	fmt.Println("Stopping music")

	if vi, ok := voiceInstances[guildId]; ok {

		vi.stop = true
		vi.voiceConnection.Disconnect()
	}
}

func skipSong(guildId string) {
	fmt.Println("skipping music")

	if vi, ok := voiceInstances[guildId]; ok {

		vi.skip = true
	}
}

func togglePauseSong(guildId string, toggle bool) {
	if vi, ok := voiceInstances[guildId]; ok {

		vi.pause = toggle
	}
}

// createVoiceInstance accepts both a youtube query and a server id, boots up the voice connection, and plays the track.
func createVoiceInstance(youtubeLink string, serverID string, vc *discordgo.VoiceConnection) {
	fmt.Println("Creating voice instance")
	fmt.Println(voiceInstances)
	vi := new(voiceInstance)
	voiceInstances[serverID] = vi
	vi.voiceConnection = vc

	vi.serverID = serverID
	vi.queue = lane.NewQueue()

	vi.pcmChannel = make(chan []int16, 2)
	go sendPCM(vi.voiceConnection, vi.pcmChannel)

	vi.queue.Enqueue(youtubeLink)
	vi.processQueue()
}

func (vi *voiceInstance) processQueue() {
	fmt.Println("processing queue")

	if vi.trackPlaying == false {
		for {
			vi.skip = false
			link := vi.queue.Dequeue()
			if link == nil || vi.stop == true {
				break
			}
			vi.startAudioStreamFromUrl(link.(string))
		}

		// No more tracks in queue? Cleanup.
		fmt.Println("Closing connections")
		close(vi.pcmChannel)
		delete(voiceInstances, vi.serverID)
		fmt.Println("Done")
	}
}

func (vi *voiceInstance) startAudioStreamFromUrl(url string) {
	fmt.Println("playing form url")
	vi.trackPlaying = true

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Http.Get\nerror: %s\ntarget: %s\n", err, url)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("reading answer: non 200 status code received: '%s'", err)
	}

	run = exec.Command("ffmpeg", "-i", "-", "-f", "s16le", "-ar", strconv.Itoa(frameRate), "-ac", strconv.Itoa(channels), "pipe:1")
	fmt.Println(resp.Body)
	run.Stdin = resp.Body
	stdout, err := run.StdoutPipe()
	if err != nil {
		fmt.Println("StdoutPipe Error:", err)
		return
	}

	err = run.Start()
	if err != nil {
		fmt.Println("RunStart Error:", err)
		return
	}

	// buffer used during loop below
	audiobuf := make([]int16, frameSize*channels)

	err = vi.voiceConnection.Speaking(true)

	if err != nil {
		fmt.Printf("Couldn't set speaking %s \n", err)
	}
	defer vi.voiceConnection.Speaking(false)

	fmt.Println("starting read loop")
	for {

		if vi.pause == true {
			continue
		}

		// read data from ffmpeg stdout
		err = binary.Read(stdout, binary.LittleEndian, &audiobuf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			fmt.Println("EOF")
			run.Process.Kill()
			break
		}
		if err != nil {
			fmt.Println("error reading from ffmpeg stdout :", err)
			run.Process.Kill()
			break
		}
		if vi.stop == true || vi.skip == true {
			fmt.Println("stop")
			run.Process.Kill()
			break
		}

		vi.pcmChannel <- audiobuf
	}

	fmt.Println("read loop ended")
	vi.trackPlaying = false
}

// SendPCM will receive on the provied channel encode
// received PCM data into Opus then send that to Discordgo
func sendPCM(v *discordgo.VoiceConnection, pcm <-chan []int16) {
	if pcm == nil {
		return
	}

	var err error

	opusEncoder, err := gopus.NewEncoder(frameRate, channels, gopus.Audio)

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
			// fmt.Println(fmt.Sprintf("Discordgo not ready for opus packets. %+v : %+v", v.Ready, v.OpusSend), nil)
			// Sending errors here might not be suited
			return
		}
		// send encoded opus data to the sendOpus channel
		v.OpusSend <- opus
	}
}
