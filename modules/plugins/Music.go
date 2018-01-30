package plugins

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"sync"

	"github.com/otium/ytdl"

	"github.com/layeh/gopus"
	"github.com/oleiade/lane"

	"github.com/Snakeyesz/snek-bot/utils"
	"github.com/bwmarrin/discordgo"
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
	discord      *discordgo.Session
	queue        *lane.Queue
	voice        *discordgo.VoiceConnection
	pcmChannel   chan []int16
	serverID     string
	skip         bool
	stop         bool
	trackPlaying bool
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

		// join the users channel
		voiceConnection, err := utils.JoinUserVoiceChat(session, msg)
		if err != nil {
			utils.AlertUserOfError(err, session, msg)
			return
		}

		video, err := ytdl.GetVideoInfo(content)
		if err != nil {
			utils.AlertUserOfError(err, session, msg)
			return
		}

		for _, format := range video.Formats {
			if format.AudioEncoding == "opus" || format.AudioEncoding == "aac" || format.AudioEncoding == "vorbis" {
				data, err := video.GetDownloadURL(format)
				if err != nil {
					fmt.Println(err)
				}

				downloadLink := data.String()
				fmt.Println(downloadLink)
				// go playAudioFile(url, guild, channel, "youtube")
				channel, err := session.State.Channel(msg.ChannelID)
				go createVoiceInstance(downloadLink, channel.GuildID, voiceConnection)
				return
			}
		}

	}

	// If the message is "pong" reply with "Ping!"
	if command == "stop" {
		// session.ChannelMessageSend(msg.ChannelID, "Ping!")
	}
}

// createVoiceInstance accepts both a youtube query and a server id, boots up the voice connection, and plays the track.
func createVoiceInstance(youtubeLink string, serverID string, vc *discordgo.VoiceConnection) {
	vi := new(voiceInstance)
	voiceInstances[serverID] = vi
	vi.voice = vc

	fmt.Println("Connecting Voice...")
	vi.serverID = serverID
	vi.queue = lane.NewQueue()

	vi.pcmChannel = make(chan []int16, 2)
	go SendPCM(vi.voice, vi.pcmChannel)

	vi.queue.Enqueue(youtubeLink)
	vi.processQueue()
}

func (vi *voiceInstance) processQueue() {
	if vi.trackPlaying == false {
		for {
			vi.skip = false
			link := vi.queue.Dequeue()
			if link == nil || vi.stop == true {
				break
			}
			vi.playVideo(link.(string))
		}

		// No more tracks in queue? Cleanup.
		fmt.Println("Closing connections")
		close(vi.pcmChannel)
		vi.voice.Close()
		delete(voiceInstances, vi.serverID)
		fmt.Println("Done")
	}
}

func (vi *voiceInstance) playVideo(url string) {
	vi.trackPlaying = true

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Http.Get\nerror: %s\ntarget: %s\n", err, url)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Print(resp.StatusCode)
		log.Printf("reading answer: non 200 status code received: '%s'", err)
	}

	run = exec.Command("ffmpeg", "-i", "-", "-f", "s16le", "-ar", strconv.Itoa(frameRate), "-ac", strconv.Itoa(channels), "pipe:1")
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

	vi.voice.Speaking(true)
	defer vi.voice.Speaking(false)

	for {
		// read data from ffmpeg stdout
		err = binary.Read(stdout, binary.LittleEndian, &audiobuf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			fmt.Println("error reading from ffmpeg stdout :", err)
			break
		}
		if vi.stop == true || vi.skip == true {
			run.Process.Kill()
			break
		}
		vi.pcmChannel <- audiobuf
	}

	vi.trackPlaying = false
}

// SendPCM will receive on the provied channel encode
// received PCM data into Opus then send that to Discordgo
func SendPCM(v *discordgo.VoiceConnection, pcm <-chan []int16) {
	mu.Lock()
	if sendpcm || pcm == nil {
		mu.Unlock()
		return
	}
	sendpcm = true
	mu.Unlock()
	defer func() { sendpcm = false }()

	opusEncoder, err := gopus.NewEncoder(frameRate, channels, gopus.Audio)
	if err != nil {
		fmt.Println("NewEncoder Error:", err)
		return
	}

	for {
		// read pcm from chan, exit if channel is closed.
		recv, ok := <-pcm
		if !ok {
			fmt.Println("PCM Channel closed.")
			return
		}

		// try encoding pcm frame with Opus
		opus, err := opusEncoder.Encode(recv, frameSize, maxBytes)
		if err != nil {
			fmt.Println("Encoding Error:", err)
			return
		}

		if v.Ready == false || v.OpusSend == nil {
			fmt.Printf("Discordgo not ready for opus packets. %+v : %+v", v.Ready, v.OpusSend)
			return
		}
		// send encoded opus data to the sendOpus channel
		v.OpusSend <- opus
	}
}
