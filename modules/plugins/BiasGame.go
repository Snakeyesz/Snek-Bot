package plugins

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math/rand"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/Snakeyesz/snek-bot/models"
	"github.com/Snakeyesz/snek-bot/modules/plugins/biasgame"
	"github.com/Snakeyesz/snek-bot/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/nfnt/resize"
	"github.com/sethgrid/pester"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

type BiasGame struct{}

type biasChoice struct {
	// info directly from google drive
	fileName       string
	driveId        string
	webViewLink    string
	webContentLink string
	gender         string

	// image
	biasImages []image.Image

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
	idolsRemaining   int
	lastRoundMessage *discordgo.Message
	readyForReaction bool   // used to make sure multiple reactions aren't counted
	gender           string // girl, boy, mixed

	// a map of fileName => image array position. This is used to make sure that when a random image is selected for a game, that the same image is still used throughout the game
	gameImageIndex map[string]int
}

const (
	DRIVE_SEARCH_TEXT   = "\"%s\" in parents and (mimeType = \"image/gif\" or mimeType = \"image/jpeg\" or mimeType = \"image/png\" or mimeType = \"application/vnd.google-apps.folder\")"
	GIRLS_FOLDER_ID     = "1CIM6yrvZOKn_R-qWYJ6pISHyq-JQRkja"
	BOYS_FOLDER_ID      = "1psrhQQaV0kwPhAMtJ7LYT2SWgLoyDb-J"
	MISC_FOLDER_ID      = "1-HdvH5fiOKuZvPPVkVMILZxkjZKv9x_x"
	IMAGE_RESIZE_HEIGHT = 150
	LEFT_ARROW_EMOJI    = "⬅"
	RIGHT_ARROW_EMOJI   = "➡"
	ZERO_WIDTH_SPACE    = "\u200B"
	BOT_OWNER_ID        = "273639623324991489"
)

// used to determine if game is ready after a bot restart
var gameIsReady = false

// misc images
var versesImage image.Image
var winnerBracket image.Image
var shadowBorder image.Image
var crown image.Image

var allBiasChoices []*biasChoice
var currentBiasGames map[string]*singleBiasGame
var allowedGameSizes map[int]bool
var biasGameGenders map[string]string

func (b *BiasGame) InitPlugin() {

	currentBiasGames = make(map[string]*singleBiasGame)

	allowedGameSizes = map[int]bool{
		10:  true, // for dev only, remove when game is live
		32:  true,
		64:  true,
		128: true,
		256: true,
	}

	biasGameGenders = map[string]string{
		"boy":   "boy",
		"boys":  "boy",
		"girl":  "girl",
		"girls": "girl",
		"mixed": "mixed",
	}

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
			resizedImage := resize.Resize(0, IMAGE_RESIZE_HEIGHT+30, img, resize.Lanczos3)
			versesImage = resizedImage

		case "topEightBracket.png":

			fmt.Println("loading verses top eight bracket image")
			winnerBracket = img
		case "shadow-border.png":

			fmt.Println("loading shadow border image")
			resizedImage := resize.Resize(0, IMAGE_RESIZE_HEIGHT+30, img, resize.Lanczos3)
			shadowBorder = resizedImage
		case "crown.png":

			fmt.Println("loading crown image")
			resizedImage := resize.Resize(IMAGE_RESIZE_HEIGHT/2, 0, img, resize.Lanczos3)
			crown = resizedImage
		}
	}

	// append crown to top eight
	bracketImage := image.NewRGBA(winnerBracket.Bounds())
	draw.Draw(bracketImage, winnerBracket.Bounds(), winnerBracket, image.Point{0, 0}, draw.Src)
	draw.Draw(bracketImage, crown.Bounds().Add(image.Pt(230, 5)), crown, image.ZP, draw.Over)
	winnerBracket = bracketImage.SubImage(bracketImage.Rect)

	// set up suggestions channel
	biasgame.InitSuggestionChannel()

	// this line should always be last in this function
	gameIsReady = true
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
	if gameIsReady == false {
		utils.SendMessage(msg.ChannelID, "biasgame.game.game-not-ready")
		return
	}

	commandArgs := strings.Fields(content)

	if len(commandArgs) == 0 {

		singleGame := createOrGetSinglePlayerGame(msg, "girl", 32)
		singleGame.sendBiasGameRound()

	} else if commandArgs[0] == "stats" {
		// stats
		displayBiasGameStats(msg, content)

	} else if commandArgs[0] == "suggest" {

		// create map of idols and there group
		groupIdolMap := make(map[string][]string)
		for _, bias := range allBiasChoices {
			groupIdolMap[bias.groupName] = append(groupIdolMap[bias.groupName], bias.biasName)
		}

		biasgame.ProcessImageSuggestion(msg, content, groupIdolMap)

	} else if commandArgs[0] == "current" {

		displayCurrentGameStats(msg)

	} else if commandArgs[0] == "idols" {

		listIdolsInGame(msg)

	} else if commandArgs[0] == "refresh-images" {

		// check if the user is the bot owner
		if msg.Author.ID == BOT_OWNER_ID {

			message, _ := utils.SendMessage(msg.ChannelID, "biasgame.refresh.refresing")
			refreshBiasChoices()

			cache.GetDiscordSession().ChannelMessageDelete(msg.ChannelID, message.ID)
			utils.SendMessage(msg.ChannelID, "biasgame.refresh.refresh-done")
		} else {
			utils.SendMessage(msg.ChannelID, "biasgame.refresh.not-bot-owner")
		}

	} else if gameSize, err := strconv.Atoi(commandArgs[0]); err == nil {

		// check if the game size the user wants is valid
		if allowedGameSizes[gameSize] {
			singleGame := createOrGetSinglePlayerGame(msg, "girl", gameSize)
			singleGame.sendBiasGameRound()
		} else {
			utils.SendMessage(msg.ChannelID, "biasgame.game.invalid-game-size")
			return
		}

	} else if gameGender, ok := biasGameGenders[commandArgs[0]]; ok {

		// check if the game size the user wants is valid
		if len(commandArgs) == 2 {

			gameSize, _ := strconv.Atoi(commandArgs[1])
			if allowedGameSizes[gameSize] {
				singleGame := createOrGetSinglePlayerGame(msg, gameGender, gameSize)
				singleGame.sendBiasGameRound()
			} else {
				utils.SendMessage(msg.ChannelID, "biasgame.game.invalid-game-size")
				return
			}
		} else {
			singleGame := createOrGetSinglePlayerGame(msg, gameGender, 32)
			singleGame.sendBiasGameRound()
		}

	}

}

// Called whenever a reaction is added to any message
func (b *BiasGame) ActionOnReactionAdd(reaction *discordgo.MessageReactionAdd) {
	if gameIsReady == false {
		return
	}

	// confirm the reaction was added to a message for one bias games
	if game, ok := currentBiasGames[reaction.UserID]; ok == true {

		// check if reaction was added to the message of the game
		if game.lastRoundMessage.ID == reaction.MessageID && game.readyForReaction == true {

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
				game.idolsRemaining--

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

					// record game stats
					go recordGameStats(game)

					// end the game. delete from current games
					delete(currentBiasGames, game.user.ID)

				} else {

					// save the last 8 for the chart
					if len(game.biasQueue) == 8 {
						game.topEight = game.biasQueue
					}

					// Sleep a time bit to allow other users to see what was chosen.
					// This creates conversation while the game is going and makes it a overall better experience
					//
					//   This will also allow me to call out and harshly judge players who don't choose nayoung.
					time.Sleep(time.Second / 5)

					game.sendBiasGameRound()
				}

			}
		}
	}

	// check if the reaction was added to a paged message
	if pagedMessage := utils.GetPagedMessage(reaction.MessageID); pagedMessage != nil {
		pagedMessage.UpdateMessagePage(reaction)
	}

	// check if this was a reaction to a idol suggestion.
	//  if it was accepted an image will be returned to be added to the biasChoices
	suggestedImageDriveFile := biasgame.CheckSuggestionReaction(reaction)
	if suggestedImageDriveFile != nil {
		addDriveFileToAllBiases(suggestedImageDriveFile)
	}
}

// sendBiasGameRound will send the message for the round
func (g *singleBiasGame) sendBiasGameRound() {
	if g == nil {
		return
	}

	// if a round message has been sent, delete before sending the next one
	if g.lastRoundMessage != nil {
		go g.deleteLastGameRoundMessage()
	}

	// combine first bias image with the "vs" image, then combine that image with 2nd bias image
	img1 := g.biasQueue[0].getRandomBiasImage(g)
	img2 := g.biasQueue[1].getRandomBiasImage(g)

	img1 = giveImageShadowBorder(img1, 15, 15)
	img2 = giveImageShadowBorder(img2, 15, 15)

	img1 = utils.CombineTwoImages(img1, versesImage)
	finalImage := utils.CombineTwoImages(img1, img2)

	// create round message
	messageString := fmt.Sprintf("%s\nIdols Remaining: %d\n%s %s vs %s %s",
		g.user.Mention(),
		g.idolsRemaining,
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
	go cache.GetDiscordSession().MessageReactionAdd(g.channelID, fileSendMsg.ID, RIGHT_ARROW_EMOJI)

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
		14: image.Pt(182, 53),

		13: image.Pt(358, 271),
		12: image.Pt(81, 271),

		11: image.Pt(443, 409),
		10: image.Pt(305, 409),
		9:  image.Pt(167, 409),
		8:  image.Pt(29, 409),

		7: image.Pt(478, 517),
		6: image.Pt(419, 517),
		5: image.Pt(340, 517),
		4: image.Pt(281, 517),
		3: image.Pt(202, 517),
		2: image.Pt(143, 517),
		1: image.Pt(64, 517),
		0: image.Pt(5, 517),
	}

	// get last 7 from winners array and combine with topEight array
	winners := g.roundWinners[len(g.roundWinners)-7 : len(g.roundWinners)]
	bracketInfo := append(g.topEight, winners...)

	// create final image with the bounds of the winner bracket
	bracketImage := image.NewRGBA(winnerBracket.Bounds())
	draw.Draw(bracketImage, winnerBracket.Bounds(), winnerBracket, image.Point{0, 0}, draw.Src)

	// populate winner brackent image
	for i, bias := range bracketInfo {
		resizeMap := map[int]uint{
			14: 165,
			13: 90, 12: 90,
			11: 60, 10: 60, 9: 60, 8: 60,
		}

		// adjust images sizing according to placement
		resizeTo := uint(50)

		if newResizeVal, ok := resizeMap[i]; ok {
			resizeTo = newResizeVal
		}

		ri := resize.Resize(0, resizeTo, bias.getRandomBiasImage(g), resize.Lanczos3)

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

// will return a random image for the bias,
//  if an image has already been chosen for the given game and bias thenit will use that one
func (b *biasChoice) getRandomBiasImage(g *singleBiasGame) image.Image {
	var imageIndex int

	// check if a random image for the idol has already been chosen for this game
	//  also make sure that biasimages array contains the index. it may have been changed due to a refresh from googledrive
	if imagePos, ok := g.gameImageIndex[b.fileName]; ok && len(b.biasImages) > imagePos {
		imageIndex = imagePos
	} else {
		imageIndex = rand.Intn(len(b.biasImages))
		g.gameImageIndex[b.fileName] = imageIndex
	}

	return b.biasImages[imageIndex]
}

// createSinglePlayerGame will setup a singleplayer game for the user
func createOrGetSinglePlayerGame(msg *discordgo.Message, gameGender string, gameSize int) *singleBiasGame {
	var singleGame *singleBiasGame
	fmt.Println("game gender: ", gameGender)

	// check if the user has a current game already going.
	// if so update the channel id for the game incase the user tried starting the game from another server
	if game, ok := currentBiasGames[msg.Author.ID]; ok {

		game.channelID = msg.ChannelID
		singleGame = game
	} else {
		var biasChoices []*biasChoice

		// if this isn't a mixed game then filter all choices by the gender
		if gameGender != "mixed" {

			for _, bias := range allBiasChoices {
				if bias.gender == gameGender {
					fmt.Println("included biases: ", bias.groupName, bias.biasName, bias.gender)
					biasChoices = append(biasChoices, bias)
				}
			}
		} else {
			biasChoices = allBiasChoices
		}

		// confirm we have enough biases to choose from for the game size this should be
		if len(biasChoices) < gameSize {
			utils.SendMessage(msg.ChannelID, "biasgame.game.not-enough-idols")
			return nil
		}

		// create new game
		singleGame = &singleBiasGame{
			user:             msg.Author,
			channelID:        msg.ChannelID,
			idolsRemaining:   gameSize,
			readyForReaction: false,
			gender:           gameGender,
		}
		singleGame.gameImageIndex = make(map[string]int)

		// get random biases for the game
		usedIndexs := make(map[int]bool)
		for true {
			randomIndex := rand.Intn(len(biasChoices))

			if usedIndexs[randomIndex] == false {
				usedIndexs[randomIndex] = true
				singleGame.biasQueue = append(singleGame.biasQueue, biasChoices[randomIndex])

				if len(singleGame.biasQueue) == gameSize {
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

	// get idol image from google drive
	girlFiles := getFilesFromDriveFolder(GIRLS_FOLDER_ID)
	boyFiles := getFilesFromDriveFolder(BOYS_FOLDER_ID)
	allFiles := append(girlFiles, boyFiles...)

	if len(allFiles) > 0 {
		var wg sync.WaitGroup
		mux := new(sync.Mutex)

		// clear all biases before refresh
		var tempAllBiases []*biasChoice

		fmt.Println("Loading Files:", len(allFiles))
		for _, file := range allFiles {
			if !strings.HasPrefix(file.Name, "P") && !strings.HasPrefix(file.Name, "T") {
				continue
			}
			wg.Add(1)

			go func(file *drive.File) {
				defer wg.Done()

				newBiasChoice, err := makeBiasChoiceFromDriveFile(file)
				if err != nil {
					return
				}

				mux.Lock()
				defer mux.Unlock()

				// if the bias already exists, then just add this picture to the image array for the idol
				for _, currentBias := range tempAllBiases {
					if currentBias.fileName == newBiasChoice.fileName {
						currentBias.biasImages = append(currentBias.biasImages, newBiasChoice.biasImages[0])
						return
					}
				}

				tempAllBiases = append(tempAllBiases, newBiasChoice)
			}(file)
		}
		wg.Wait()
		fmt.Println("Amount of idols loaded: ", len(tempAllBiases))
		allBiasChoices = tempAllBiases

	} else {
		fmt.Println("No bias files found.")
	}
}

// getFilesFromDriveFolder
func getFilesFromDriveFolder(folderId string) []*drive.File {
	driveService := cache.GetGoogleDriveService()

	// get girls image from google drive
	results, err := driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, folderId)).Fields(googleapi.Field("nextPageToken, files(name, id, parents, webViewLink, webContentLink)")).PageSize(1000).Do()
	if err != nil {
		fmt.Printf("Error getting google drive files from folderid: %s\n%s\n", folderId, err.Error())
		return nil
	}
	allFiles := results.Files

	// retry for more bias images if needed
	pageToken := results.NextPageToken
	for pageToken != "" {
		results, err = driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, folderId)).Fields(googleapi.Field("nextPageToken, files(name, id, parents, webViewLink, webContentLink)")).PageSize(1000).PageToken(pageToken).Do()
		pageToken = results.NextPageToken
		if len(results.Files) > 0 {
			allFiles = append(allFiles, results.Files...)
		} else {
			break
		}
	}

	return allFiles
}

// makeBiasChoiceFromDriveFile
func makeBiasChoiceFromDriveFile(file *drive.File) (*biasChoice, error) {
	res, err := pester.Get(file.WebContentLink)
	if err != nil {
		fmt.Println("get error: ", err.Error())
		return nil, err
	}

	// decode image
	img, imgErr := utils.DecodeImage(res.Body)
	if imgErr != nil {
		fmt.Printf("error decoding image %s:\n %s", file.Name, imgErr)
		return nil, imgErr
	}

	resizedImage := resize.Resize(0, IMAGE_RESIZE_HEIGHT, img, resize.Lanczos3)

	// get bias name and group name from file name
	groupBias := strings.TrimSuffix(file.Name, filepath.Ext(file.Name))

	var gender string
	if file.Parents[0] == GIRLS_FOLDER_ID {
		gender = "girl"
	} else {
		gender = "boy"
	}

	newBiasChoice := &biasChoice{
		fileName:       file.Name,
		driveId:        file.Id,
		webViewLink:    file.WebViewLink,
		webContentLink: file.WebContentLink,
		groupName:      strings.Split(groupBias, "_")[0],
		biasName:       strings.Split(groupBias, "_")[1],
		biasImages:     []image.Image{resizedImage},
		gender:         gender,
	}

	return newBiasChoice, nil
}

// addDriveFileToAllBiases will take a drive file, convert it to a bias object,
//   and add it to allBiasChoices or add a new image if the idol already exists
func addDriveFileToAllBiases(file *drive.File) {
	newBiasChoice, err := makeBiasChoiceFromDriveFile(file)
	if err != nil {
		return
	}

	// if the bias already exists, then just add this picture to the image array for the idol
	for _, currentBias := range allBiasChoices {
		if currentBias.fileName == newBiasChoice.fileName {
			currentBias.biasImages = append(currentBias.biasImages, newBiasChoice.biasImages[0])
			return
		}
	}

	allBiasChoices = append(allBiasChoices, newBiasChoice)
}

// listIdolsInGame will list all idols that can show up in the biasgame
func listIdolsInGame(msg *discordgo.Message) {

	// create map of idols and there group
	groupIdolMap := make(map[string][]string)
	for _, bias := range allBiasChoices {
		groupIdolMap[bias.groupName] = append(groupIdolMap[bias.groupName], bias.biasName)
	}

	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED, // blueish
		Author: &discordgo.MessageEmbedAuthor{
			Name: fmt.Sprintf("All Idols Available In Bias Game (%d total)", len(allBiasChoices)),
		},
	}

	// make fields for each group and the idols in the group.
	for group, idols := range groupIdolMap {

		// sort idols by name
		sort.Slice(idols, func(i, j int) bool {
			return idols[i] < idols[j]
		})

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   group,
			Value:  strings.Join(idols, ", "),
			Inline: false,
		})
	}

	// sort fields by group name
	sort.Slice(embed.Fields, func(i, j int) bool {
		return strings.ToLower(embed.Fields[i].Name) < strings.ToLower(embed.Fields[j].Name)
	})

	utils.SendPagedMessage(msg, embed, 10)
}

// displayCurrentGameStats will list the rounds and round winners of a currently running game
func displayCurrentGameStats(msg *discordgo.Message) {

	blankField := &discordgo.MessageEmbedField{
		Name:   ZERO_WIDTH_SPACE,
		Value:  ZERO_WIDTH_SPACE,
		Inline: true,
	}

	// find currently running game for the user or a mention if one exists
	userPlayingGame := msg.Author
	if len(msg.Mentions) > 0 {
		userPlayingGame = msg.Mentions[0]
	}

	if game, ok := currentBiasGames[userPlayingGame.ID]; ok {

		embed := &discordgo.MessageEmbed{
			Color: 0x0FADED, // blueish
			Author: &discordgo.MessageEmbedAuthor{
				Name: fmt.Sprintf("%s - Current Game Info\n", userPlayingGame.Username),
			},
		}

		// for i := 0; i < len(game.roundWinners); i++ {
		for i := len(game.roundWinners) - 1; i >= 0; i-- {

			fieldName := fmt.Sprintf("Round %d:", i+1)
			if len(game.roundWinners) == i+1 {
				fieldName = "Last Round:"
			}

			message := fmt.Sprintf("W: %s %s\nL: %s %s\n",
				game.roundWinners[i].groupName,
				game.roundWinners[i].biasName,
				game.roundLosers[i].groupName,
				game.roundLosers[i].biasName)

			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fieldName,
				Value:  message,
				Inline: true,
			})
		}

		// notify user if no rounds have been played in the game yet
		if len(embed.Fields) == 0 {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   "No Rounds",
				Value:  utils.Geti18nText("biasgame.current.no-rounds-played"),
				Inline: true,
			})
		}

		// this is to corrent alignment
		if len(embed.Fields)%3 == 1 {
			embed.Fields = append(embed.Fields, blankField)
			embed.Fields = append(embed.Fields, blankField)
		} else if len(embed.Fields)%3 == 2 {
			embed.Fields = append(embed.Fields, blankField)
		}

		utils.SendPagedMessage(msg, embed, 12)
	} else {
		utils.SendMessage(msg.ChannelID, "biasgame.current.no-running-game")
	}

}

// recordGameStats will record the winner, round winners/losers, and other misc stats of a game
func recordGameStats(game *singleBiasGame) {

	// get guildID from game channel
	channel, _ := cache.GetDiscordSession().State.Channel(game.channelID)
	guild, err := cache.GetDiscordSession().State.Guild(channel.GuildID)
	if err != nil {
		fmt.Println("Error getting guild when recording stats")
		return
	}

	// create a bias game entry
	biasGameEntry := &models.SingleBiasGameEntry{
		ID:           "",
		UserID:       game.user.ID,
		GuildID:      guild.ID,
		Gender:       game.gender,
		RoundWinners: compileGameWinnersLosers(game.roundWinners),
		RoundLosers:  compileGameWinnersLosers(game.roundLosers),
		GameWinner: models.BiasEntry{
			Name:      game.gameWinnerBias.biasName,
			GroupName: game.gameWinnerBias.groupName,
			Gender:    game.gameWinnerBias.gender,
		},
	}

	utils.MongoDBInsert(models.BiasGameTable, biasGameEntry)
}

// displayBiasGameStats will display stats for the bias game based on the stats message
func displayBiasGameStats(msg *discordgo.Message, statsMessage string) {
	results, iconURL, targetName := getStatsResults(msg, statsMessage)

	// check if the user has stats and give a message if they do not
	resultCount, err := results.Count()
	if err != nil || resultCount == 0 {
		utils.SendMessage(msg.ChannelID, "biasgame.stats.no-stats")
		return
	}

	statsTitle := ""
	countsHeader := ""
	totalGames, err := results.Count()
	if err != nil {
		fmt.Println("Err getting stat result count: ", err.Error())
		return
	}

	// loop through the results and compile a map of [biasgroup biasname]number of occurences
	items := results.Iter()
	biasCounts := make(map[string]int)
	game := models.SingleBiasGameEntry{}
	for items.Next(&game) {
		groupAndName := ""

		if strings.Contains(statsMessage, "rounds won") {

			// round winners
			for _, rWinner := range game.RoundWinners {

				if strings.Contains(statsMessage, "group") {
					statsTitle = "Rounds Won in Bias Game by Group"
					groupAndName = fmt.Sprintf("%s", rWinner.GroupName)
				} else {
					statsTitle = "Rounds Won in Bias Game"
					groupAndName = fmt.Sprintf("**%s** %s", rWinner.GroupName, rWinner.Name)
				}
				biasCounts[groupAndName] += 1
			}

			countsHeader = "Rounds Won"

		} else if strings.Contains(statsMessage, "rounds lost") {

			// round losers
			for _, rLoser := range game.RoundLosers {

				if strings.Contains(statsMessage, "group") {
					statsTitle = "Rounds Lost in Bias Game by Group"
					groupAndName = fmt.Sprintf("%s", rLoser.GroupName)
				} else {
					statsTitle = "Rounds Lost in Bias Game"
					groupAndName = fmt.Sprintf("**%s** %s", rLoser.GroupName, rLoser.Name)
				}
				biasCounts[groupAndName] += 1
			}

			statsTitle = "Rounds Lost in Bias Game"
			countsHeader = "Rounds Lost"
		} else {

			// game winners
			if strings.Contains(statsMessage, "group") {
				statsTitle = "Bias Game Winners by Group"
				groupAndName = fmt.Sprintf("%s", game.GameWinner.GroupName)
			} else {
				statsTitle = "Bias Game Winners"
				groupAndName = fmt.Sprintf("**%s** %s", game.GameWinner.GroupName, game.GameWinner.Name)
			}

			biasCounts[groupAndName] += 1
			countsHeader = "Games Won"

		}
	}

	// add total games to the stats header message
	statsTitle = fmt.Sprintf("%s (%d games)", statsTitle, totalGames)

	sendStatsMessage(msg, statsTitle, countsHeader, biasCounts, iconURL, targetName)
}

// getStatsResults will get the stats results based on the stats message
func getStatsResults(msg *discordgo.Message, statsMessage string) (*mgo.Query, string, string) {
	iconURL := ""
	targetName := ""

	queryParams := bson.M{}
	// user/server/global checks
	if strings.Contains(statsMessage, "server") {

		guild, err := utils.GetGuildFromMessage(msg)
		if err != nil {
			// todo: a message here or something i guess?
		}

		iconURL = discordgo.EndpointGuildIcon(guild.ID, guild.Icon)
		targetName = "Server"
		queryParams["guildid"] = guild.ID
	} else if strings.Contains(statsMessage, "global") {
		iconURL = cache.GetDiscordSession().State.User.AvatarURL("512")
		targetName = "Global"

	} else if strings.Contains(statsMessage, "@") {
		iconURL = msg.Mentions[0].AvatarURL("512")
		targetName = msg.Mentions[0].Username

		queryParams["userid"] = msg.Mentions[0].ID
	} else {
		iconURL = msg.Author.AvatarURL("512")
		targetName = msg.Author.Username

		queryParams["userid"] = msg.Author.ID
	}

	if strings.Contains(statsMessage, "boy") || strings.Contains(statsMessage, "boys") {
		queryParams["gamewinner.gender"] = "boy"
	} else if strings.Contains(statsMessage, "girl") || strings.Contains(statsMessage, "girls") {
		queryParams["gamewinner.gender"] = "girl"
	}

	// date checks
	// if strings.Contains(statsMessage, "today") {
	// 	// dateCheck := bson.NewObjectIdWithTime()
	// 	messageTime, _ := msg.Timestamp.Parse()

	// 	from := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, messageTime.Location())
	// 	to := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 23, 59, 59, 0, messageTime.Location())

	// 	fromId := bson.NewObjectIdWithTime(from)
	// 	toId := bson.NewObjectIdWithTime(to)

	// 	queryParams["_id"] = bson.M{"$gte": fromId, "$lt": toId}
	// }

	return utils.MongoDBSearch(models.BiasGameTable, queryParams), iconURL, targetName
}

// complieGameStats will convert records from database into a:
// 		map[int number of occurentces]string group or biasnames comma delimited
// 		will also return []int of the sorted unique counts for reliable looping later
func complieGameStats(records map[string]int) (map[int][]string, []int) {

	// use map of counts to compile a new map of [unique occurence amounts]biasnames
	var uniqueCounts []int
	compiledData := make(map[int][]string)
	for k, v := range records {
		// store unique counts so the map can be "sorted"
		if _, ok := compiledData[v]; !ok {
			uniqueCounts = append(uniqueCounts, v)
		}

		compiledData[v] = append(compiledData[v], k)
	}

	// sort biggest to smallest
	sort.Sort(sort.Reverse(sort.IntSlice(uniqueCounts)))

	return compiledData, uniqueCounts
}

func sendStatsMessage(msg *discordgo.Message, title string, countLabel string, data map[string]int, iconURL string, targetName string) {

	embed := &discordgo.MessageEmbed{
		Color: 0x0FADED, // blueish
		Author: &discordgo.MessageEmbedAuthor{
			Name:    fmt.Sprintf("%s - %s\n", targetName, title),
			IconURL: iconURL,
		},
	}

	// convert data to map[num of occurences]delimited biases
	compiledData, uniqueCounts := complieGameStats(data)
	for _, count := range uniqueCounts {

		// sort biases by group
		sort.Slice(compiledData[count], func(i, j int) bool {
			return compiledData[count][i] < compiledData[count][j]
		})

		joinedNames := strings.Join(compiledData[count], ", ")

		if len(joinedNames) < 1024 {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("%s - %d", countLabel, count),
				Value:  joinedNames,
				Inline: false,
			})

		} else {

			// for a specific count, split into multiple fields of at max 40 names
			dataForCount := compiledData[count]
			namesPerField := 40
			breaker := true
			for breaker {

				var namesForField string
				if len(dataForCount) >= namesPerField {
					namesForField = strings.Join(dataForCount[:namesPerField], ", ")
					dataForCount = dataForCount[namesPerField:]
				} else {
					namesForField = strings.Join(dataForCount, ", ")
					breaker = false
				}

				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:   fmt.Sprintf("%s - %d", countLabel, count),
					Value:  namesForField,
					Inline: false,
				})

			}
		}

	}

	// send paged message with 5 fields per page
	utils.SendPagedMessage(msg, embed, 5)
}

// compileGameWinnersLosers will loop through the biases and convert them to []models.BiasEntry
func compileGameWinnersLosers(biases []*biasChoice) []models.BiasEntry {

	var biasEntries []models.BiasEntry
	for _, bias := range biases {
		biasEntries = append(biasEntries, models.BiasEntry{
			Name:      bias.biasName,
			GroupName: bias.groupName,
			Gender:    bias.gender,
		})
	}

	return biasEntries
}

// giveImageShadowBorder give the round image a shadow border
func giveImageShadowBorder(img image.Image, offsetX int, offsetY int) image.Image {
	rgba := image.NewRGBA(shadowBorder.Bounds())
	draw.Draw(rgba, shadowBorder.Bounds(), shadowBorder, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, img.Bounds().Add(image.Pt(offsetX, offsetY)), img, image.ZP, draw.Over)
	return rgba.SubImage(rgba.Rect)
}
