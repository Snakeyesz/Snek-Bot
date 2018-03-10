package biasgame

import (
	"fmt"
	"image"

	"github.com/sethgrid/pester"
	"google.golang.org/api/drive/v3"

	"github.com/davecgh/go-spew/spew"

	"github.com/Snakeyesz/snek-bot/models"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/Snakeyesz/snek-bot/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/mgutz/str"
)

const (
	IMAGE_SUGGESTION_CHANNEL = "420049316615553026"

	CHECKMARK_EMOJI = "✅"
	X_EMOJI         = "❌"

	MAX_IMAGE_SIZE = 2000 // 2000px x 2000px
)

var suggestionQueue []*models.BiasGameSuggestion
var suggestionEmbedMessageId string // id of the embed message suggestions are accepted/denied

func InitSuggestionChannel() {
	// suggestionQueue = make(map[string]*idolSuggestion)

	// when the bot starts, delete any past bot messages from the suggestion channel and make the embed
	var messagesToDelete []string
	messagesInChannel, _ := cache.GetDiscordSession().ChannelMessages(IMAGE_SUGGESTION_CHANNEL, 100, "", "", "")
	// spew.Dump(messagesInChannel)
	for _, msg := range messagesInChannel {
		messagesToDelete = append(messagesToDelete, msg.ID)
		// if msg.Author.ID == cache.GetDiscordSession().State.User.ID {
		// }
	}

	err := cache.GetDiscordSession().ChannelMessagesBulkDelete(IMAGE_SUGGESTION_CHANNEL, messagesToDelete)
	if err != nil {
		fmt.Println("Error deleting messages: ", err.Error())
	}

	// create the first embed
	updateCurrentSuggestionEmbed()
}

// processImageSuggestion
func ProcessImageSuggestion(msg *discordgo.Message, msgContent string) {
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

	// validate suggestion args
	if len(suggestionArgs) != 4 {
		utils.SendMessage(msg.ChannelID, invalidArgsMessage)
		return
	} else if suggestionArgs[0] != "girl" && suggestionArgs[0] != "boy" {
		utils.SendMessage(msg.ChannelID, invalidArgsMessage)
		return
	}

	// validate url image
	resp, err := pester.Get(suggestionArgs[3])
	if err != nil {
		utils.SendMessage(msg.ChannelID, "biasgame.suggestion.invalid-url")
		return
	}
	defer resp.Body.Close()
	fmt.Println("content type: ", resp.Header.Get("Content-type"))

	// make sure image is png
	if resp.Header.Get("Content-type") != "image/png" {
		utils.SendMessage(msg.ChannelID, "biasgame.suggestion.not-png-format")
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

	// send ty message
	utils.SendMessage(msg.ChannelID, "biasgame.suggestion.thanks-for-suggestion")

	// create suggetion
	suggestion := &models.BiasGameSuggestion{
		UserID:     msg.Author.ID,
		ChannelID:  msg.ChannelID,
		Gender:     suggestionArgs[0],
		GrouopName: suggestionArgs[1],
		Name:       suggestionArgs[2],
		ImageURL:   suggestionArgs[3],
	}

	// save suggetion to database and memory
	suggestionQueue = append(suggestionQueue, suggestion)
	utils.MongoDBInsert(models.BiasGameSuggestionsTable, suggestion)
	updateCurrentSuggestionEmbed()
}

// CheckSuggestionReaction will check if the reaction was added to a suggestion message
func CheckSuggestionReaction(reaction *discordgo.MessageReactionAdd) {
	if CHECKMARK_EMOJI != reaction.Emoji.Name && X_EMOJI != reaction.Emoji.Name {
		return
	}

	if reaction.MessageID == suggestionEmbedMessageId {
		cs := suggestionQueue[0]

		dmChannel, err := cache.GetDiscordSession().UserChannelCreate(cs.UserID)
		if err != nil {
			return
		}

		// update current page based on direction
		if CHECKMARK_EMOJI == reaction.Emoji.Name {
			utils.SendMessage(dmChannel.ID, fmt.Sprintf("Your idol suggestion for `%s %s` has been APPROVED! Thank you again for the suggestion.", cs.GrouopName, cs.Name))

			res, err := pester.Get(cs.ImageURL)

			var folderId string
			if cs.Gender == "boy" {
				folderId = "1psrhQQaV0kwPhAMtJ7LYT2SWgLoyDb-J"
			} else {
				folderId = "1CIM6yrvZOKn_R-qWYJ6pISHyq-JQRkja"
			}

			file_meta := &drive.File{Name: fmt.Sprintf("%s_%s", cs.GrouopName, cs.Name), Parents: []string{folderId}}
			_, err = cache.GetGoogleDriveService().Files.Create(file_meta).Media(res.Body).Do()

			if err != nil {
				fmt.Println("error uploading to google: ", err.Error())
			}
		}

		if X_EMOJI == reaction.Emoji.Name {
			utils.SendMessage(dmChannel.ID, fmt.Sprintf("Unfortunatly idol suggestion for `%s %s` has been denied.", cs.GrouopName, cs.Name))
		}

		// delete first suggestion and process queue again
		suggestionQueue = suggestionQueue[1:]
		updateCurrentSuggestionEmbed()
	}
}

func updateCurrentSuggestionEmbed() {
	var embed *discordgo.MessageEmbed

	spew.Dump(suggestionQueue)
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

		suggestedBy, err := cache.GetDiscordSession().User(cs.UserID)

		suggestedFromText := "No Guild Info"
		suggestedFromCh, err := cache.GetDiscordSession().Channel(cs.ChannelID)
		suggestedFrom, err := cache.GetDiscordSession().Guild(suggestedFromCh.GuildID)
		if err == nil {
			suggestedFromText = fmt.Sprintf("%s", suggestedFrom.Name)
		}

		// res, err := pester.Get(cs.ImageURL)
		// if err != nil {
		// 	// todo: discard submission
		// 	return nil
		// }

		embed = &discordgo.MessageEmbed{
			Color: 0x0FADED, // blueish
			Author: &discordgo.MessageEmbedAuthor{
				Name: fmt.Sprintf("Suggestions in queue: %d", len(suggestionQueue)),
			},
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Idol Name",
					Value:  cs.Name,
					Inline: true,
				},
				{
					Name:   "Idol Group",
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
				{
					Name:   "Image URL",
					Value:  cs.ImageURL,
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

// recordImageSuggestion records suggestion to the dedicated channel
// func (s *models.BiasGameSuggestion) updateSuggestionEmbed() {

// 	channel, _ := cache.GetDiscordSession().State.Channel(msg.ChannelID)
// 	guild, _ := utils.GetGuildFromMessage(msg)

// 	// validation passed, put suggestion in suggestion channel
// 	suggestionMessage := fmt.Sprintf("Idol Suggestion for Bias Game\n\nFrom Server: %s | %s\nFrom Channel: %s | %s\nFrom User: %s\nIdol Gender: %s\nIdol Group: %s\nIdol Name: %s\nImage URL: <%s>",
// 		guild.Name,
// 		guild.ID,
// 		channel.Name,
// 		channel.ID,
// 		msg.Author.Username,
// 		suggestionArgs[0],
// 		suggestionArgs[1],
// 		suggestionArgs[2],
// 		suggestionArgs[3])

// 	message, _ := utils.SendMessage(IMAGE_SUGGESTION_CHANNEL, suggestionMessage)

// 	cache.GetDiscordSession().MessageReactionAdd(message.ChannelID, message.ID, THUMB_UP_EMOJI)
// 	cache.GetDiscordSession().MessageReactionAdd(message.ChannelID, message.ID, THUMB_DOWN_EMOJI)
// }
