package plugins

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math/rand"
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
	unusedBiasCoices []*biasChoice
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
var allBiasChoices []*biasChoice
var currentBiasGames map[string]*singleBiasGame

func (b *BiasGame) InitPlugin() {

	currentBiasGames = make(map[string]*singleBiasGame)

	// load all bias images and information
	refreshBiasChoices()

	// load the verses image
	driveService := cache.GetGoogleDriveService()
	results, err := driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, MISC_FOLDER_ID)).Fields("nextPageToken, files").Do()
	if err != nil {
		fmt.Println(err)
	}

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

		resizedImage := resize.Resize(0, IMAGE_RESIZE_HEIGHT, img, resize.Lanczos3)
		versesImage = resizedImage

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

	singleGame := createOrGetSinglePlayerGame(msg)
	singleGame.sendBiasRound()
}

// Called whenever a reaction is added to any message
func (b *BiasGame) ActionOnReactionAdd(reaction *discordgo.MessageReactionAdd) {
	fmt.Println(currentBiasGames)
	fmt.Println("user id", reaction.UserID)

	// confirm the reaction was added to a message for one bias games
	if game, ok := currentBiasGames[reaction.UserID]; ok == true {
		fmt.Println("game found")

		// used to make sure multple quick reactions to trigger unexpected behavior
		if !game.readyForReaction {
			fmt.Println("game not ready for reaction")
			return
		}

		if game.lastRoundMessage.ID != reaction.MessageID {
			fmt.Println("message ids don't match")
			return
		}

		winnerIndex := 0
		loserIndex := 0
		validReaction := false
		// if the user reacts with an arrow,
		//    record winners and losers remove the first 2 biases from unused
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
			game.unusedBiasCoices = append(game.unusedBiasCoices, game.unusedBiasCoices[winnerIndex])
			game.roundLosers = append(game.roundLosers, game.unusedBiasCoices[loserIndex])
			game.unusedBiasCoices = game.unusedBiasCoices[2:]
			cache.GetDiscordSession().ChannelMessageDelete(game.channelID, game.lastRoundMessage.ID)
			game.roundsRemaining--
			game.sendBiasRound()
		}
		fmt.Println("emoji id", reaction.Emoji.ID)
		fmt.Println("emoji name", reaction.Emoji.Name)
		fmt.Println("emoji apiname", reaction.Emoji.APIName())
	} else {
		fmt.Println("no game found")
	}
}

// sendBiasRound will send the message for the round
func (g *singleBiasGame) sendBiasRound() {

	start := time.Now()
	// combine first bias image with the "vs" image, then combine that image with 2nd bias image
	img1 := g.unusedBiasCoices[0].biasImage
	img2 := g.unusedBiasCoices[1].biasImage
	img1 = utils.CombineTwoImages(img1, versesImage)
	finalImage := utils.CombineTwoImages(img1, img2)

	fmt.Println("image concat execution time: ", time.Since(start).Nanoseconds())

	start = time.Now()
	// create round message
	messageString := fmt.Sprintf("%s\nRounds Remaining: %d\n%s %s vs %s %s",
		g.user.Mention(),
		g.roundsRemaining,
		g.unusedBiasCoices[0].groupName,
		g.unusedBiasCoices[0].biasName,
		g.unusedBiasCoices[1].groupName,
		g.unusedBiasCoices[1].biasName)
	fmt.Println("string execution time: ", time.Since(start).Nanoseconds())

	start = time.Now()
	// encode the combined image and compress it
	buf := new(bytes.Buffer)
	encoder := new(png.Encoder)
	encoder.CompressionLevel = -2 // -2 compression is best speed, -3 is best compression but end result isn't worth the slower encoding
	encoder.Encode(buf, finalImage)
	myReader := bytes.NewReader(buf.Bytes())
	fmt.Println("encoding execution time: ", time.Since(start).Nanoseconds())

	start = time.Now()
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
	fmt.Println("send messages execution time: ", time.Since(start).Nanoseconds())

}

// createSinglePlayerGame will setup a singleplayer game for the user
func createOrGetSinglePlayerGame(msg *discordgo.Message) *singleBiasGame {
	var singleGame *singleBiasGame

	// check if the user has a current game already going.
	// if so update the channel id for the game incase the user tried starting the game from another server
	if game, ok := currentBiasGames[msg.Author.ID]; ok {

		singleGame = game
	} else {
		// create new game
		singleGame = &singleBiasGame{
			user:             msg.Author,
			channelID:        msg.ChannelID,
			roundsRemaining:  32,
			readyForReaction: false,
		}

		// get random biases for the game
		usedIndexs := make(map[int]bool)
		for i := 0; true; i++ {
			randomIndex := rand.Intn(len(allBiasChoices))

			if usedIndexs[randomIndex] == false {
				usedIndexs[randomIndex] = true
				singleGame.unusedBiasCoices = append(singleGame.unusedBiasCoices, allBiasChoices[randomIndex])

				if len(singleGame.unusedBiasCoices) == 32 {
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
	results, err := driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, GIRLS_FOLDER_ID)).PageSize(50).Fields("nextPageToken, files").Do()
	if err != nil {
		fmt.Println(err)
	}

	if len(results.Files) > 0 {

		var wg sync.WaitGroup
		mux := new(sync.Mutex)

		fmt.Println("Files:", len(results.Files))
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

				// fmt.Println("File Information: ")
				// fmt.Println("\t ", file.Name)
				// fmt.Println("\t ", file.Id)
				// fmt.Println("\t ", file.WebViewLink)
				// fmt.Println("\t ", file.WebContentLink)
			}(file)
		}
		wg.Wait()
	} else {
		fmt.Println("No bias files found.")
	}
}
