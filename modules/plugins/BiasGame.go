package plugins

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nfnt/resize"

	"google.golang.org/api/drive/v3"

	"github.com/Snakeyesz/snek-bot/utils"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/bwmarrin/discordgo"
)

const (
	DRIVE_SEARCH_TEXT = "\"%s\" in parents and (mimeType = \"image/gif\" or mimeType = \"image/jpeg\" or mimeType = \"image/png\" or mimeType = \"application/vnd.google-apps.folder\")"
	GIRLS_FOLDER_ID   = "1CIM6yrvZOKn_R-qWYJ6pISHyq-JQRkja"
	MISC_FOLDER_ID    = "1-HdvH5fiOKuZvPPVkVMILZxkjZKv9x_x"
)

var versesImage image.Image
var biasChoices []*biasChoiceInfo

type biasChoiceInfo struct {
	// info directly from google drive
	fileName       string
	driveId        string
	webViewLink    string
	webContentLink string

	// image info
	biasImage image.Image

	// bias info
	biasName  string
	groupName string
}

type BiasGame struct{}

func (b *BiasGame) InitPlugin() {

	refreshBiasChoices()

	// load the verses image
	driveService := cache.GetGoogleDriveService()
	results, err := driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, MISC_FOLDER_ID)).Fields("nextPageToken, files").Do()
	if err != nil {
		fmt.Println(err)
	}

	if len(results.Files) > 0 {
		for _, file := range results.Files {
			if file.Name != "verses.png" {
				continue
			}

			res, err := http.Get(file.WebContentLink)
			if err != nil {
				return
			}
			img, _, err := image.Decode(res.Body)
			if err != nil {
				continue
			}

			resizedImage := resize.Resize(0, 150, img, resize.Lanczos3)

			versesImage = resizedImage

		}
	} else {
		fmt.Println("No misc files found.")
	}
}

// Will validate if the pass command entered is used for this plugin
func (b *BiasGame) ValidateCommand(command string) bool {
	validCommands := []string{"biasgame"}

	for _, v := range validCommands {
		if v == command {
			return true
		}
	}

	return false
}

// Main Entry point for the plugin
func (b *BiasGame) Action(command string, content string, msg *discordgo.Message, session *discordgo.Session) {

	fmt.Println("command")
	var images []image.Image

	for i, biasChoice := range biasChoices {
		if i > 1 {
			break
		}
		fmt.Println("File Information: ")
		fmt.Println("\t ", biasChoice.fileName)
		fmt.Println("\t ", biasChoice.driveId)
		fmt.Println("\t ", biasChoice.webViewLink)
		fmt.Println("\t ", biasChoice.webContentLink)

		images = append(images, biasChoice.biasImage)
	}

	images[0] = combineTwoImage(images[0], versesImage)
	finalImage := combineTwoImage(images[0], images[1])

	buf := new(bytes.Buffer)
	encoder := new(png.Encoder)
	encoder.CompressionLevel = 2
	encoder.Encode(buf, finalImage)

	messageString := msg.Author.Mention() +
		"\nRounds Remaining: 32\n" +
		biasChoices[0].groupName + " " + biasChoices[0].biasName + " vs " +
		biasChoices[1].groupName + " " + biasChoices[1].biasName

	fmt.Println("sending file")
	myReader := bytes.NewReader(buf.Bytes())
	fileSendMsg, err := utils.SendFile(msg.ChannelID, "combined_pic.png", myReader, messageString)
	if err != nil {
		return
	}

	fmt.Println("adding reactions")
	cache.GetDiscordSession().MessageReactionAdd(fileSendMsg.ChannelID, fileSendMsg.ID, "⬅")
	cache.GetDiscordSession().MessageReactionAdd(fileSendMsg.ChannelID, fileSendMsg.ID, "➡")
}

// refreshes the list of bias choices
func refreshBiasChoices() {
	driveService := cache.GetGoogleDriveService()
	results, err := driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, GIRLS_FOLDER_ID)).PageSize(10).Fields("nextPageToken, files").Do()
	if err != nil {
		fmt.Println(err)
	}

	if len(results.Files) > 0 {

		var wg sync.WaitGroup
		mux := new(sync.Mutex)

		fmt.Println("Files:", len(results.Files))
		time.Sleep(5 * time.Second)
		for _, file := range results.Files {
			wg.Add(1)

			go func(file *drive.File) {
				defer wg.Done()

				res, err := http.Get(file.WebContentLink)
				if err != nil {
					return
				}

				// decode image
				img, _, _ := image.Decode(res.Body)

				resizedImage := resize.Resize(0, 150, img, resize.Lanczos3)

				// get bias name and group name from file name
				groupBias := strings.Split(file.Name, ".")[0]

				biasChoice := &biasChoiceInfo{
					fileName:       file.Name,
					driveId:        file.Id,
					webViewLink:    file.WebViewLink,
					webContentLink: file.WebContentLink,
					biasImage:      resizedImage,
					groupName:      strings.Split(groupBias, "_")[0],
					biasName:       strings.Split(groupBias, "_")[1],
				}
				mux.Lock()
				biasChoices = append(biasChoices, biasChoice)
				mux.Unlock()

				fmt.Println("File Information: ")
				fmt.Println("\t ", file.Name)
				fmt.Println("\t ", file.Id)
				fmt.Println("\t ", file.WebViewLink)
				fmt.Println("\t ", file.WebContentLink)
			}(file)
		}
		wg.Wait()
	} else {
		fmt.Println("No bias files found.")
	}
}

func combineTwoImage(img1, img2 image.Image) image.Image {

	//starting position of the second image (bottom left)
	sp2 := image.Point{img1.Bounds().Dx(), 0}

	//new rectangle for the second image
	r2 := image.Rectangle{sp2, sp2.Add(img2.Bounds().Size())}

	//rectangle for the big image
	r := image.Rectangle{image.Point{0, 0}, r2.Max}
	rgba := image.NewRGBA(r)

	draw.Draw(rgba, img1.Bounds(), img1, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, r2, img2, image.Point{0, 0}, draw.Src)

	return rgba.SubImage(r)
}
