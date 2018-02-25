package plugins

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/googleapi"

	"github.com/nfnt/resize"

	"google.golang.org/api/drive/v3"

	"github.com/Snakeyesz/snek-bot/utils"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/bwmarrin/discordgo"
)

type BiasGame struct{}

type biasChoice struct {
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

type singleBiasGame struct {
	user             *discordgo.User
	channelID        string
	roundLosers      []*biasChoice
	roundWinners     []*biasChoice
	biasQueue        []*biasChoice
	topEight         []*biasChoice
	gameWinnerBias   *biasChoice
	roundsRemaining  int
	lastRoundMessage *discordgo.Message
	readyForReaction bool // used to make sure multiple reactions aren't counted
}

const (
	DRIVE_SEARCH_TEXT = "\"%s\" in parents and (mimeType = \"image/gif\" or mimeType = \"image/jpeg\" or mimeType = \"image/png\" or mimeType = \"application/vnd.google-apps.folder\")"
	GIRLS_FOLDER_ID   = "1CIM6yrvZOKn_R-qWYJ6pISHyq-JQRkja"
	MISC_FOLDER_ID    = "1-HdvH5fiOKuZvPPVkVMILZxkjZKv9x_x"

	IMAGE_RESIZE_HEIGHT = 150
	LEFT_ARROW_EMOJI    = "⬅"
	RIGHT_ARROW_EMOJI   = "➡"
)

var versesImage image.Image
var winnerBracket image.Image
var allBiasChoices []*biasChoice
var currentBiasGames map[string]*singleBiasGame
var bracketHTML string

func (b *BiasGame) InitPlugin() {

	currentBiasGames = make(map[string]*singleBiasGame)

	// load all bias images and information
	refreshBiasChoices()

	// load the verses and winnerBracket image
	driveService := cache.GetGoogleDriveService()
	results, err := driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, MISC_FOLDER_ID)).Fields("nextPageToken, files").Do()
	if err != nil {
		fmt.Println(err)
	}

	for _, file := range results.Files {
		res, err := http.Get(file.WebContentLink)
		if err != nil {
			return
		}
		img, _, err := image.Decode(res.Body)
		if err != nil {
			continue
		}

		switch file.Name {
		case "verses.png":
			fmt.Println("loading verses image")

			// resize verses image to match the bias image sizes
			resizedImage := resize.Resize(0, IMAGE_RESIZE_HEIGHT, img, resize.Lanczos3)
			versesImage = resizedImage

		case "topEightBracket - Copy.png":

			fmt.Println("loading verses top eight bracket image")
			winnerBracket = img
		}
	}

	// load bracket html
	var temp []byte
	assetsPath := cache.GetAppConfig().Path("assets_folder").Data().(string)
	temp, err = ioutil.ReadFile(assetsPath + "/BiasGame/top-8.html")
	if err != nil {
		fmt.Println(err)
	}
	bracketHTML = string(temp)
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

	singleGame := createOrGetSinglePlayerGame(msg)
	singleGame.sendBiasGameRound()

}

// Called whenever a reaction is added to any message
func (b *BiasGame) ActionOnReactionAdd(reaction *discordgo.MessageReactionAdd) {

	// confirm the reaction was added to a message for one bias games
	if game, ok := currentBiasGames[reaction.UserID]; ok == true {

		// check if reaction was added to the message of the game
		if game.lastRoundMessage.ID != reaction.MessageID {
			return
		}

		// used to make sure multple quick reactions to trigger unexpected behavior
		if !game.readyForReaction {
			return
		}

		winnerIndex := 0
		loserIndex := 0
		validReaction := false

		// check if the reaction added to the message was a left or right arrow
		if LEFT_ARROW_EMOJI == reaction.Emoji.Name {
			winnerIndex = 0
			loserIndex = 1
			validReaction = true
		} else if RIGHT_ARROW_EMOJI == reaction.Emoji.Name {
			winnerIndex = 1
			loserIndex = 0
			validReaction = true
		}

		if validReaction == true {
			game.readyForReaction = false
			game.roundsRemaining--

			// record winners and losers for stats
			game.roundLosers = append(game.roundLosers, game.biasQueue[loserIndex])
			game.roundWinners = append(game.roundWinners, game.biasQueue[winnerIndex])

			// add winner to end of bias queue and remove first two
			game.biasQueue = append(game.biasQueue, game.biasQueue[winnerIndex])
			game.biasQueue = game.biasQueue[2:]

			// if there is only one bias left, they are the winner
			if len(game.biasQueue) == 1 {

				game.gameWinnerBias = game.biasQueue[0]
				game.sendWinnerMessage()

				// TODO: record winner, all winners, and losers

				// end the game. delete from current games
				delete(currentBiasGames, game.user.ID)

			} else {

				if len(game.biasQueue) == 8 {
					game.topEight = game.biasQueue
				}
				// Sleep a time bit to allow other users to see what was chosen.
				// This creates conversation while the game is going and makes it a overall better experience
				//
				//   This will also allow me to call out and harshly judge players who don't choose nayoung.
				time.Sleep(time.Second / 3)

				game.sendBiasGameRound()
			}

		}
	}
}

// sendBiasGameRound will send the message for the round
func (g *singleBiasGame) sendBiasGameRound() {

	// if a round message has been sent, delete before sending the next one
	if g.lastRoundMessage != nil {
		g.deleteLastGameRoundMessage()
	}

	// combine first bias image with the "vs" image, then combine that image with 2nd bias image
	img1 := g.biasQueue[0].biasImage
	img2 := g.biasQueue[1].biasImage
	img1 = utils.CombineTwoImages(img1, versesImage)
	finalImage := utils.CombineTwoImages(img1, img2)

	// create round message
	messageString := fmt.Sprintf("%s\nRounds Remaining: %d\n%s %s vs %s %s",
		g.user.Mention(),
		g.roundsRemaining,
		g.biasQueue[0].groupName,
		g.biasQueue[0].biasName,
		g.biasQueue[1].groupName,
		g.biasQueue[1].biasName)

	// encode the combined image and compress it
	buf := new(bytes.Buffer)
	encoder := new(png.Encoder)
	encoder.CompressionLevel = -2 // -2 compression is best speed, -3 is best compression but end result isn't worth the slower encoding
	encoder.Encode(buf, finalImage)
	myReader := bytes.NewReader(buf.Bytes())

	// send round message
	fileSendMsg, err := utils.SendFile(g.channelID, "combined_pic.png", myReader, messageString)
	if err != nil {
		return
	}

	// add reactions
	cache.GetDiscordSession().MessageReactionAdd(g.channelID, fileSendMsg.ID, LEFT_ARROW_EMOJI)
	cache.GetDiscordSession().MessageReactionAdd(g.channelID, fileSendMsg.ID, RIGHT_ARROW_EMOJI)

	// update game state
	g.lastRoundMessage = fileSendMsg
	g.readyForReaction = true
}

// sendWinnerMessage creates the top eight brackent sends the winning message to the user
func (g *singleBiasGame) sendWinnerMessage() {

	// if a round message has been sent, delete before sending the next one
	if g.lastRoundMessage != nil {
		g.deleteLastGameRoundMessage()
	}

	// offsets of where bias images need to be placed on bracket image
	bracketImageOffsets := map[int]image.Point{
		14: image.Pt(182, 8),

		13: image.Pt(81, 226),
		12: image.Pt(358, 226),

		11: image.Pt(29, 364),
		10: image.Pt(167, 364),
		9:  image.Pt(305, 364),
		8:  image.Pt(443, 364),

		7: image.Pt(5, 472),
		6: image.Pt(64, 472),
		5: image.Pt(143, 472),
		4: image.Pt(202, 472),
		3: image.Pt(281, 472),
		2: image.Pt(340, 472),
		1: image.Pt(419, 472),
		0: image.Pt(478, 472),
	}

	// get last 7 from winners array and combine with topEight array
	winners := g.roundWinners[len(g.roundWinners)-7 : len(g.roundWinners)]
	bracketInfo := append(g.topEight, winners...)

	// create final image with the bounds of the winner bracket
	bracketImage := image.NewRGBA(winnerBracket.Bounds())
	draw.Draw(bracketImage, winnerBracket.Bounds(), winnerBracket, image.Point{0, 0}, draw.Src)

	// populate winner brackent image
	for i, bias := range bracketInfo {
		fmt.Println(i, bias.groupName, bias.biasName)

		// adjust images sizing according to placement
		resizeTo := uint(50)
		if i == 14 {
			resizeTo = 165
		} else if i == 13 || i == 12 {
			resizeTo = 90
		} else if i == 11 || i == 10 || i == 9 || i == 8 {
			resizeTo = 60
		}
		ri := resize.Resize(0, resizeTo, bias.biasImage, resize.Lanczos3)

		draw.Draw(bracketImage, ri.Bounds().Add(bracketImageOffsets[i]), ri, image.ZP, draw.Over)
	}

	// compress bracket image
	buf := new(bytes.Buffer)
	encoder := new(png.Encoder)
	encoder.CompressionLevel = -2 // -2 compression is best speed, -3 is best compression but end result isn't worth the slower encoding
	encoder.Encode(buf, bracketImage)
	myReader := bytes.NewReader(buf.Bytes())

	messageString := fmt.Sprintf("%s\nWinner: %s %s!",
		g.user.Mention(),
		g.gameWinnerBias.groupName,
		g.gameWinnerBias.biasName)

	// send message
	utils.SendFile(g.channelID, "biasgame_winner.png", myReader, messageString)
}

// deleteLastGameRoundMessage
func (g *singleBiasGame) deleteLastGameRoundMessage() {
	cache.GetDiscordSession().ChannelMessageDelete(g.lastRoundMessage.ChannelID, g.lastRoundMessage.ID)
}

// createSinglePlayerGame will setup a singleplayer game for the user
func createOrGetSinglePlayerGame(msg *discordgo.Message) *singleBiasGame {
	var singleGame *singleBiasGame

	// check if the user has a current game already going.
	// if so update the channel id for the game incase the user tried starting the game from another server
	if game, ok := currentBiasGames[msg.Author.ID]; ok {

		game.channelID = msg.ChannelID
		singleGame = game
	} else {
		// create new game
		singleGame = &singleBiasGame{
			user:             msg.Author,
			channelID:        msg.ChannelID,
			roundsRemaining:  31,
			readyForReaction: false,
		}

		// get random biases for the game
		usedIndexs := make(map[int]bool)
		for i := 0; true; i++ {
			randomIndex := rand.Intn(len(allBiasChoices))

			if usedIndexs[randomIndex] == false {
				usedIndexs[randomIndex] = true
				singleGame.biasQueue = append(singleGame.biasQueue, allBiasChoices[randomIndex])

				if len(singleGame.biasQueue) == 32 {
					break
				}
			}
		}

		// save game to current running games
		currentBiasGames[msg.Author.ID] = singleGame
	}

	return singleGame
}

// refreshes the list of bias choices
func refreshBiasChoices() {
	driveService := cache.GetGoogleDriveService()

	// get bias image from google drive
	results, err := driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, GIRLS_FOLDER_ID)).Fields(googleapi.Field("nextPageToken, files(name, id, webViewLink, webContentLink)")).PageSize(1000).Do()
	if err != nil {
		fmt.Println(err)
	}
	allFiles := results.Files

	// retry for more bias images if needed
	pageToken := results.NextPageToken
	for pageToken != "" {
		results, err = driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, GIRLS_FOLDER_ID)).Fields(googleapi.Field("nextPageToken, files(name, id, webViewLink, webContentLink)")).PageSize(1000).PageToken(pageToken).Do()
		pageToken = results.NextPageToken
		if len(results.Files) > 0 {
			allFiles = append(allFiles, results.Files...)
		} else {
			break
		}

	}

	if len(allFiles) > 0 {
		var wg sync.WaitGroup
		mux := new(sync.Mutex)

		fmt.Println("Files:", len(allFiles))
		for _, file := range allFiles {
			wg.Add(1)

			go func(file *drive.File) {
				defer wg.Done()

				res, err := http.Get(file.WebContentLink)
				if err != nil {
					return
				}

				// decode image
				img, _, _ := image.Decode(res.Body)

				resizedImage := resize.Resize(0, IMAGE_RESIZE_HEIGHT, img, resize.Lanczos3)

				// get bias name and group name from file name
				groupBias := strings.Split(file.Name, ".")[0]

				biasChoice := &biasChoice{
					fileName:       file.Name,
					driveId:        file.Id,
					webViewLink:    file.WebViewLink,
					webContentLink: file.WebContentLink,
					biasImage:      resizedImage,
					groupName:      strings.Split(groupBias, "_")[0],
					biasName:       strings.Split(groupBias, "_")[1],
				}
				mux.Lock()
				allBiasChoices = append(allBiasChoices, biasChoice)
				mux.Unlock()

			}(file)
		}
		wg.Wait()
	} else {
		fmt.Println("No bias files found.")
	}
}
