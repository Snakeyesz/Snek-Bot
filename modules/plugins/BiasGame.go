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
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/globalsign/mgo"

	"github.com/globalsign/mgo/bson"

	"github.com/Snakeyesz/snek-bot/models"

	"google.golang.org/api/googleapi"

	"github.com/nfnt/resize"

	"google.golang.org/api/drive/v3"

	"github.com/Snakeyesz/snek-bot/utils"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/Snakeyesz/snek-bot/modules/plugins/biasgame"
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
	idolsRemaining   int
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

// misc images
var versesImage image.Image
var winnerBracket image.Image
var shadowBorder image.Image

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
			resizedImage := resize.Resize(0, IMAGE_RESIZE_HEIGHT+30, img, resize.Lanczos3)
			versesImage = resizedImage

		case "topEightBracket.png":

			fmt.Println("loading verses top eight bracket image")
			winnerBracket = img
		case "shadow-border.png":

			fmt.Println("loading shadow border image")
			resizedImage := resize.Resize(0, IMAGE_RESIZE_HEIGHT+30, img, resize.Lanczos3)
			shadowBorder = resizedImage
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

	// stats
	if strings.Index(content, "stats") == 0 {
		printUserStats(msg, content)

	} else if strings.Index(content, "suggest") == 0 {

		biasgame.ProcessImageSuggestion(msg, content)
	} else if content == "" {

		singleGame := createOrGetSinglePlayerGame(msg, 32)
		singleGame.sendBiasGameRound()
	} else if gameSize, err := strconv.Atoi(content); err == nil {

		allowedGameSizes := map[int]bool{
			10:  true, // for dev only, remove when game is live
			32:  true,
			64:  true,
			128: true,
			256: true,
		}

		// check if the game size the user wants is valid
		if allowedGameSizes[gameSize] {
			singleGame := createOrGetSinglePlayerGame(msg, gameSize)
			singleGame.sendBiasGameRound()
		} else {
			utils.SendMessage(msg.ChannelID, "biasgame.game.invalid-game-size")
			return
		}

	}

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

	// check if the reaction was added to a paged message
	if pagedMessage := utils.GetPagedMessage(reaction.MessageID); pagedMessage != nil {
		pagedMessage.UpdateMessagePage(reaction)
	}

	// check if this was a reaction to a idol suggestion
	biasgame.CheckSuggestionReaction(reaction)
}

// sendBiasGameRound will send the message for the round
func (g *singleBiasGame) sendBiasGameRound() {

	// if a round message has been sent, delete before sending the next one
	if g.lastRoundMessage != nil {
		go g.deleteLastGameRoundMessage()
	}

	// combine first bias image with the "vs" image, then combine that image with 2nd bias image
	img1 := g.biasQueue[0].biasImage
	img2 := g.biasQueue[1].biasImage

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
	go cache.GetDiscordSession().MessageReactionAdd(g.channelID, fileSendMsg.ID, LEFT_ARROW_EMOJI)
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
func createOrGetSinglePlayerGame(msg *discordgo.Message, gameSize int) *singleBiasGame {
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
			idolsRemaining:   gameSize,
			readyForReaction: false,
		}

		// get random biases for the game
		usedIndexs := make(map[int]bool)
		for i := 0; true; i++ {
			randomIndex := rand.Intn(len(allBiasChoices))

			if usedIndexs[randomIndex] == false {
				usedIndexs[randomIndex] = true
				singleGame.biasQueue = append(singleGame.biasQueue, allBiasChoices[randomIndex])

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
	driveService := cache.GetGoogleDriveService()

	// get bias image from google drive
	results, err := driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, GIRLS_FOLDER_ID)).Fields(googleapi.Field("nextPageToken, files(name, id, webViewLink, webContentLink)")).PageSize(10).Do()
	if err != nil {
		fmt.Println(err)
	}
	allFiles := results.Files

	// retry for more bias images if needed
	// pageToken := results.NextPageToken
	// for pageToken != "" {
	// 	results, err = driveService.Files.List().Q(fmt.Sprintf(DRIVE_SEARCH_TEXT, GIRLS_FOLDER_ID)).Fields(googleapi.Field("nextPageToken, files(name, id, webViewLink, webContentLink)")).PageSize(1000).PageToken(pageToken).Do()
	// 	pageToken = results.NextPageToken
	// 	if len(results.Files) > 0 {
	// 		allFiles = append(allFiles, results.Files...)
	// 	} else {
	// 		break
	// 	}

	// }

	if len(allFiles) > 0 {
		var wg sync.WaitGroup
		mux := new(sync.Mutex)

		fmt.Println("Loading Files:", len(allFiles))
		for _, file := range allFiles {
			wg.Add(1)

			go func(file *drive.File) {
				defer wg.Done()

				res, err := http.Get(file.WebContentLink)
				if err != nil {
					return
				}

				// decode image
				img, _, imgErr := image.Decode(res.Body)
				if imgErr != nil {
					fmt.Printf("error decoding image %s:\n %s", file.Name, imgErr)
					return
				}

				resizedImage := resize.Resize(0, IMAGE_RESIZE_HEIGHT, img, resize.Lanczos3)

				// get bias name and group name from file name
				groupBias := strings.TrimSuffix(file.Name, filepath.Ext(file.Name))

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
		RoundWinners: compileGameWinnersLosers(game.roundWinners),
		RoundLosers:  compileGameWinnersLosers(game.roundLosers),
		GameWinner: models.BiasEntry{
			Name:      game.gameWinnerBias.biasName,
			GroupName: game.gameWinnerBias.groupName,
		},
	}

	utils.MongoDBInsert(models.BiasGameTable, biasGameEntry)
}

// printUserWinners
func printUserStats(msg *discordgo.Message, statsMessage string) {
	results, iconURL, targetName := getStatsResults(msg, statsMessage)

	// check if the user has stats and give a message if they do not
	resultCount, err := results.Count()
	if err != nil || resultCount == 0 {
		utils.SendMessage(msg.ChannelID, "biasgame.stats.no-stats")
		return
	}

	statsTitle := ""
	countsHeader := ""

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

	// date checks
	if strings.Contains(statsMessage, "today") {
		// dateCheck := bson.NewObjectIdWithTime()
		messageTime, _ := msg.Timestamp.Parse()

		from := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, messageTime.Location())
		to := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 23, 59, 59, 0, messageTime.Location())

		fromId := bson.NewObjectIdWithTime(from)
		toId := bson.NewObjectIdWithTime(to)

		queryParams["_id"] = bson.M{"$gte": fromId, "$lt": toId}
	}

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

			fmt.Println("before: ", strings.Join(compiledData[count][:40], ", "))
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("%s - %d", countLabel, count),
				Value:  strings.Join(compiledData[count][:40], ", "),
				Inline: false,
			})

			fmt.Println("after: ", strings.Join(compiledData[count][40:], ", "))
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("%s - %d", countLabel, count),
				Value:  strings.Join(compiledData[count][40:], ", "),
				Inline: false,
			})
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
		})
	}

	return biasEntries
}

func giveImageShadowBorder(img image.Image, offsetX int, offsetY int) image.Image {

	rgba := image.NewRGBA(shadowBorder.Bounds())
	draw.Draw(rgba, shadowBorder.Bounds(), shadowBorder, image.Point{0, 0}, draw.Src)
	draw.Draw(rgba, img.Bounds().Add(image.Pt(offsetX, offsetY)), img, image.ZP, draw.Over)

	return rgba.SubImage(rgba.Rect)
}
