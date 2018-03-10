package biasgame

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"regexp"
	"strings"
	"time"

	"github.com/globalsign/mgo/bson"
	"github.com/sethgrid/pester"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"

	"github.com/Snakeyesz/snek-bot/models"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/Snakeyesz/snek-bot/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/mgutz/str"
)

const (
	IMAGE_SUGGESTION_CHANNEL = "420049316615553026"

	CHECKMARK_EMOJI    = "✅"
	X_EMOJI            = "❌"
	QUESTIONMARK_EMOJI = "❓"

	MAX_IMAGE_SIZE = 2000 // 2000px x 2000px

	GIRLS_FOLDER_ID = "1CIM6yrvZOKn_R-qWYJ6pISHyq-JQRkja"
	BOYS_FOLDER_ID  = "1psrhQQaV0kwPhAMtJ7LYT2SWgLoyDb-J"
)

var suggestionQueue []*models.BiasGameSuggestionEntry
var suggestionEmbedMessageId string // id of the embed message where suggestions are accepted/denied
var genderFolderMap map[string]string

func InitSuggestionChannel() {

	// when the bot starts, delete any past bot messages from the suggestion channel and make the embed
	var messagesToDelete []string
	messagesInChannel, _ := cache.GetDiscordSession().ChannelMessages(IMAGE_SUGGESTION_CHANNEL, 100, "", "", "")
	for _, msg := range messagesInChannel {
		messagesToDelete = append(messagesToDelete, msg.ID)
		// if msg.Author.ID == cache.GetDiscordSession().State.User.ID {
		// }
	}

	err := cache.GetDiscordSession().ChannelMessagesBulkDelete(IMAGE_SUGGESTION_CHANNEL, messagesToDelete)
	if err != nil {
		fmt.Println("Error deleting messages: ", err.Error())
	}

	// load unresolved suggestions and create the first embed
	loadUnresolvedSuggestions()
	updateCurrentSuggestionEmbed()

	genderFolderMap = map[string]string{
		"boy":  BOYS_FOLDER_ID,
		"girl": GIRLS_FOLDER_ID,
	}
}

// processImageSuggestion
func ProcessImageSuggestion(msg *discordgo.Message, msgContent string, groupIdolMap map[string][]string) {
	invalidArgsMessage := "Invalid suggestion arguments. \n\n" +
		"Suggestion must be done with the following format:\n```!biasgame suggest [boy/girl] \"group name\" \"idol name\" [url to image]```\n" +
		"For Example:\n```!biasgame suggest girl \"PRISTIN\" \"Nayoung\" https://cdn.discordapp.com/attachments/420049316615553026/420056295618510849/unknown.png```\n\n"

	defer func() {
		if r := recover(); r != nil {
			// fmt.Println("Panic: ", r)
			utils.SendMessage(msg.ChannelID, invalidArgsMessage)
		}
	}()

	// ToArgv can panic, need to catch that
	suggestionArgs := str.ToArgv(msgContent)[1:]
	var suggestedImageUrl string

	// validate suggestion arg amount.
	if len(msg.Attachments) == 1 {
		if len(suggestionArgs) != 3 {
			utils.SendMessage(msg.ChannelID, invalidArgsMessage)
			return
		}
		suggestedImageUrl = msg.Attachments[0].URL
	} else {
		if len(suggestionArgs) != 4 {
			utils.SendMessage(msg.ChannelID, invalidArgsMessage)
			return
		}
		suggestedImageUrl = suggestionArgs[3]
	}

	// set gender to lowercase and check if its valid
	suggestionArgs[0] = strings.ToLower(suggestionArgs[0])
	if suggestionArgs[0] != "girl" && suggestionArgs[0] != "boy" {
		utils.SendMessage(msg.ChannelID, invalidArgsMessage)
		return
	}

	// validate url image
	resp, err := pester.Get(suggestedImageUrl)
	if err != nil {
		utils.SendMessage(msg.ChannelID, "biasgame.suggestion.invalid-url")
		return
	}
	defer resp.Body.Close()
	fmt.Println("content type: ", resp.Header.Get("Content-type"))

	// make sure image is png or jpeg
	if resp.Header.Get("Content-type") != "image/png" && resp.Header.Get("Content-type") != "image/jpeg" {
		utils.SendMessage(msg.ChannelID, "biasgame.suggestion.not-png-or-jpeg")
		return
	}

	// attempt to decode the image, if we can't there may be something wrong with the image submitted
	suggestedImage, _, errr := image.Decode(resp.Body)
	if errr != nil {
		utils.SendMessage(msg.ChannelID, "biasgame.suggestion.invalid-url")
		fmt.Println("image decode error: ", err)
		return
	}

	// Check height and width are equal
	if suggestedImage.Bounds().Dy() != suggestedImage.Bounds().Dx() {
		utils.SendMessage(msg.ChannelID, "biasgame.suggestion.image-not-square")
		return
	}

	// Validate size of image
	if suggestedImage.Bounds().Dy() > MAX_IMAGE_SIZE {
		utils.SendMessage(msg.ChannelID, "biasgame.suggestion.image-to-big")
		return
	}

	// validate group and idol name have no double quotes or underscores
	if strings.ContainsAny(suggestionArgs[1]+suggestionArgs[2], "\"_") {
		utils.SendMessage(msg.ChannelID, "biasgame.suggestion.invalid-group-or-idol")
		return
	}

	// check if the group suggested matches a current group. do loose comparison
	groupMatch := false
	idolMatch := false
	reg, _ := regexp.Compile("[^a-zA-Z0-9]+")
	for k, v := range groupIdolMap {
		curGroup := strings.ToLower(reg.ReplaceAllString(k, ""))
		sugGroup := strings.ToLower(reg.ReplaceAllString(suggestionArgs[1], ""))
		fmt.Println("groups: ", curGroup, sugGroup)

		// if groups match, set the suggested group to the current group
		if curGroup == sugGroup {
			groupMatch = true
			suggestionArgs[1] = k

			// check if the idols name matches
			for _, idolName := range v {
				curName := strings.ToLower(reg.ReplaceAllString(idolName, ""))
				sugName := strings.ToLower(reg.ReplaceAllString(suggestionArgs[2], ""))

				if curName == sugName {
					idolMatch = true
					suggestionArgs[2] = idolName
					break
				}
			}
			break
		}
	}

	// send ty message
	utils.SendMessage(msg.ChannelID, "biasgame.suggestion.thanks-for-suggestion")

	// create suggetion
	suggestion := &models.BiasGameSuggestionEntry{
		UserID:     msg.Author.ID,
		ChannelID:  msg.ChannelID,
		Gender:     suggestionArgs[0],
		GrouopName: suggestionArgs[1],
		Name:       suggestionArgs[2],
		ImageURL:   suggestedImageUrl,
		GroupMatch: groupMatch,
		IdolMatch:  idolMatch,
	}

	// save suggetion to database and memory
	suggestionQueue = append(suggestionQueue, suggestion)
	utils.MongoDBInsert(models.BiasGameSuggestionsTable, suggestion)
	updateCurrentSuggestionEmbed()
}

// CheckSuggestionReaction will check if the reaction was added to a suggestion message
func CheckSuggestionReaction(reaction *discordgo.MessageReactionAdd) *drive.File {
	var approvedFiles *drive.File
	var userResponseMessage string

	// check if the reaction added was valid
	if CHECKMARK_EMOJI != reaction.Emoji.Name && X_EMOJI != reaction.Emoji.Name {
		return nil
	}

	// check if the reaction was added to the suggestion embed message
	if reaction.MessageID == suggestionEmbedMessageId {
		cs := suggestionQueue[0]

		// update current page based on direction
		if CHECKMARK_EMOJI == reaction.Emoji.Name {

			// make call to get suggestion image
			res, err := pester.Get(cs.ImageURL)
			if err != nil {
				msg, _ := utils.SendMessage(IMAGE_SUGGESTION_CHANNEL, "biasgame.suggestion.could-not-decode")
				go utils.DeleteImageWithDelay(msg, time.Second*15)
				return nil
			}

			approvedImage, err := utils.DecodeImage(res.Body)
			if err != nil {
				msg, _ := utils.SendMessage(IMAGE_SUGGESTION_CHANNEL, "biasgame.suggestion.could-not-decode")
				go utils.DeleteImageWithDelay(msg, time.Second*15)
				return nil
			}

			buf := new(bytes.Buffer)
			encoder := new(png.Encoder)
			encoder.CompressionLevel = -2 // -2 compression is best speed
			encoder.Encode(buf, approvedImage)
			myReader := bytes.NewReader(buf.Bytes())

			// upload image to google drive
			file_meta := &drive.File{Name: fmt.Sprintf("%s_%s.png", cs.GrouopName, cs.Name), Parents: []string{genderFolderMap[cs.Gender]}}
			approvedFiles, err = cache.GetGoogleDriveService().Files.Create(file_meta).Media(myReader).Fields(googleapi.Field("name, id, parents, webViewLink, webContentLink")).Do()
			if err != nil {
				fmt.Println("error: ", err.Error())
				msg, _ := utils.SendMessage(IMAGE_SUGGESTION_CHANNEL, "biasgame.suggestion.drive-upload-failed")
				go utils.DeleteImageWithDelay(msg, time.Second*15)
				return nil
			}

			// set image accepted image
			userResponseMessage = fmt.Sprintf("Your image suggestion for `%s %s` has been APPROVED! Thank you again for the suggestion.", cs.GrouopName, cs.Name)

			// update record
			cs.Status = "approved"
			utils.MongoDBUpdate(models.BiasGameSuggestionsTable, cs.ID, cs)

		} else if X_EMOJI == reaction.Emoji.Name {

			// image was denied
			userResponseMessage = fmt.Sprintf("Unfortunatly idol suggestion for `%s %s` has been denied.", cs.GrouopName, cs.Name)

			// update record
			cs.Status = "denied"
			utils.MongoDBUpdate(models.BiasGameSuggestionsTable, cs.ID, cs)
		}

		dmChannel, err := cache.GetDiscordSession().UserChannelCreate(cs.UserID)
		utils.SendMessage(dmChannel.ID, userResponseMessage)
		if err != nil {
			return nil
		}

		// delete first suggestion and process queue again
		suggestionQueue = suggestionQueue[1:]
		go updateCurrentSuggestionEmbed()
	}

	return approvedFiles
}

func updateCurrentSuggestionEmbed() {
	var embed *discordgo.MessageEmbed

	if len(suggestionQueue) == 0 {

		embed = &discordgo.MessageEmbed{
			Color: 0x0FADED, // blueish
			Author: &discordgo.MessageEmbedAuthor{
				Name: "No suggestions in queue",
			},
		}

	} else {
		// current suggestion
		cs := suggestionQueue[0]

		// get info of user who suggested image
		suggestedBy, err := cache.GetDiscordSession().User(cs.UserID)

		// get guild and channel info it was suggested from
		suggestedFromText := "No Guild Info"
		suggestedFromCh, err := cache.GetDiscordSession().Channel(cs.ChannelID)
		suggestedFrom, err := cache.GetDiscordSession().Guild(suggestedFromCh.GuildID)
		if err == nil {
			suggestedFromText = fmt.Sprintf("%s | #%s", suggestedFrom.Name, suggestedFromCh.Name)
		}

		// if the group name and idol name were matched show a checkmark, otherwise show a question mark
		groupNameDisplay := "Group Name"
		if cs.GroupMatch == true {
			groupNameDisplay += " " + CHECKMARK_EMOJI
		} else {
			groupNameDisplay += " " + QUESTIONMARK_EMOJI
		}
		idolNameDisplay := "Idol Name"
		if cs.IdolMatch == true {
			idolNameDisplay += " " + CHECKMARK_EMOJI
		} else {
			idolNameDisplay += " " + QUESTIONMARK_EMOJI
		}

		embed = &discordgo.MessageEmbed{
			Color: 0x0FADED, // blueish
			Author: &discordgo.MessageEmbedAuthor{
				Name: fmt.Sprintf("Suggestions in queue: %d", len(suggestionQueue)),
			},
			Image: &discordgo.MessageEmbedImage{
				URL: cs.ImageURL,
			},
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   idolNameDisplay,
					Value:  cs.Name,
					Inline: true,
				},
				{
					Name:   groupNameDisplay,
					Value:  cs.GrouopName,
					Inline: true,
				},
				{
					Name:   "Gender",
					Value:  cs.Gender,
					Inline: true,
				},
				{
					Name:   "Suggested By",
					Value:  fmt.Sprintf("%s", suggestedBy.Mention()),
					Inline: true,
				},
				{
					Name:   "Suggested From",
					Value:  suggestedFromText,
					Inline: true,
				},
				{
					Name:   "Timestamp",
					Value:  cs.ID.Time().Format("Jan 2, 2006 3:04pm (MST)"),
					Inline: true,
				},
			},
		}
	}

	// send or edit embed message
	var embedMsg *discordgo.Message
	if suggestionEmbedMessageId == "" {
		embedMsg, _ = utils.SendEmbed(IMAGE_SUGGESTION_CHANNEL, embed)
		suggestionEmbedMessageId = embedMsg.ID
	} else {
		embedMsg, _ = utils.EditEmbed(IMAGE_SUGGESTION_CHANNEL, suggestionEmbedMessageId, embed)
	}

	// delete any reactions on message and then reset them if needed
	cache.GetDiscordSession().MessageReactionsRemoveAll(IMAGE_SUGGESTION_CHANNEL, embedMsg.ID)
	if len(suggestionQueue) > 0 {
		cache.GetDiscordSession().MessageReactionAdd(IMAGE_SUGGESTION_CHANNEL, embedMsg.ID, CHECKMARK_EMOJI)
		cache.GetDiscordSession().MessageReactionAdd(IMAGE_SUGGESTION_CHANNEL, embedMsg.ID, X_EMOJI)
	}
}

// loadUnresolvedSuggestions
func loadUnresolvedSuggestions() {
	queryParams := bson.M{}

	queryParams["status"] = ""

	results := utils.MongoDBSearch(models.BiasGameSuggestionsTable, queryParams)

	resultCount, err := results.Count()
	if err != nil || resultCount == 0 {
		return
	}

	results.All(&suggestionQueue)
	// items := results.Iter()
	// game := models.SingleBiasGameEntry{}
	// for items.Next(&game) {
}
