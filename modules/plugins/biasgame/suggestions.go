package biasgame

import (
	"fmt"
	"image"
	"net/http"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/Snakeyesz/snek-bot/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/mgutz/str"
)

const (
	IMAGE_SUGGESTION_CHANNEL = "420049316615553026"

	THUMB_UP_EMOJI   = "👍"
	THUMB_DOWN_EMOJI = "👎"
)

// messageid to suggestion command
var idolSuggestions map[string]*idolSuggestion

type idolSuggestion struct {
	messageID         string // id of the suggestion message in suggestion channel
	channelID         string // channel suggestion was made from
	userID            string //user who made the suggestion
	groupName         string
	idolName          string
	imageURL          string
	suggestionMessage string
}

func init() {
	idolSuggestions = make(map[string]*idolSuggestion)
}

// processImageSuggestion
func ProcessImageSuggestion(msg *discordgo.Message, msgContent string) {

	suggestionArgs := str.ToArgv(msgContent)[1:]

	invalidArgsMessage := "Invalid suggestion arguments. \n\n" +
		"Suggestion must be done with the following format:\n```!biasgame suggest [boy/girl] \"group name\" \"idol name\" [url to image]```\n" +
		"For Example:\n```!biasgame suggest girl \"PRISTIN\" \"Nayoung\" https://cdn.discordapp.com/attachments/420049316615553026/420056295618510849/unknown.png```\n\n"

	// validate suggestion args
	if len(suggestionArgs) != 4 {
		utils.SendMessage(msg.ChannelID, invalidArgsMessage)
		return
	} else if suggestionArgs[0] != "girl" && suggestionArgs[0] != "boy" {
		utils.SendMessage(msg.ChannelID, invalidArgsMessage)
		return
	}

	// validate url image
	resp, err := http.Get(suggestionArgs[3])
	if err != nil {
		utils.SendMessage(msg.ChannelID, "biasgame.suggestion.invalid-url")
		return
	}
	defer resp.Body.Close()

	suggestedImage, _, errr := image.Decode(resp.Body)
	if errr != nil {
		utils.SendMessage(msg.ChannelID, "biasgame.suggestion.invalid-url")
		fmt.Println("image decode error: ", err)
		return
	}

	// Check height and width
	if suggestedImage.Bounds().Dy() != suggestedImage.Bounds().Dx() {
		utils.SendMessage(msg.ChannelID, "biasgame.suggestion.image-not-square")
		return
	}

	channel, _ := cache.GetDiscordSession().State.Channel(msg.ChannelID)
	guild, _ := utils.GetGuildFromMessage(msg)

	// validation passed, put suggestion in suggestion channel
	suggestionMessage := fmt.Sprintf("Idol Suggestion for Bias Game\n\nFrom Server: %s | %s\nFrom Channel: %s | %s\nFrom User: %s\nIdol Gender: %s\nIdol Group: %s\nIdol Name: %s\nImage URL: <%s>",
		guild.Name,
		guild.ID,
		channel.Name,
		channel.ID,
		msg.Author.Username,
		suggestionArgs[0],
		suggestionArgs[1],
		suggestionArgs[2],
		suggestionArgs[3])

	utils.SendMessage(msg.ChannelID, "biasgame.suggestion.thanks-for-suggestion")
	message, _ := utils.SendMessage(IMAGE_SUGGESTION_CHANNEL, suggestionMessage)

	suggestion := &idolSuggestion{
		messageID:         message.ID,
		userID:            msg.Author.ID,
		channelID:         msg.ChannelID,
		groupName:         suggestionArgs[1],
		idolName:          suggestionArgs[2],
		imageURL:          suggestionArgs[3],
		suggestionMessage: suggestionMessage,
	}

	idolSuggestions[message.ID] = suggestion

	cache.GetDiscordSession().MessageReactionAdd(message.ChannelID, message.ID, THUMB_UP_EMOJI)
	cache.GetDiscordSession().MessageReactionAdd(message.ChannelID, message.ID, THUMB_DOWN_EMOJI)
}

// CheckSuggestionReaction will check if the reaction was added to a suggestion message
func CheckSuggestionReaction(reaction *discordgo.MessageReactionAdd) {
	if THUMB_UP_EMOJI != reaction.Emoji.Name && THUMB_DOWN_EMOJI != reaction.Emoji.Name {
		return
	}

	if suggestion, ok := idolSuggestions[reaction.MessageID]; ok {
		dmChannel, err := cache.GetDiscordSession().UserChannelCreate(suggestion.userID)
		if err != nil {
			return
		}

		var suggestionMessage string

		// update current page based on direction
		if THUMB_UP_EMOJI == reaction.Emoji.Name {
			utils.SendMessage(dmChannel.ID, fmt.Sprintf("Your idol suggestion for `%s %s` has been APPROVED! Thank you again for the suggestion.", suggestion.groupName, suggestion.idolName))
			suggestionMessage = suggestion.suggestionMessage + "\nStatus: APPROVED"
		}

		if THUMB_DOWN_EMOJI == reaction.Emoji.Name {
			utils.SendMessage(dmChannel.ID, fmt.Sprintf("Unfortunatly idol suggestion for `%s %s` has been denied.", suggestion.groupName, suggestion.idolName))
			suggestionMessage = suggestion.suggestionMessage + "\nStatus: DENIED"
		}

		// may return error due to permissions, don't need to catch it
		cache.GetDiscordSession().MessageReactionsRemoveAll(reaction.ChannelID, reaction.MessageID)

		// update suggestion message
		_, err = cache.GetDiscordSession().ChannelMessageEdit(reaction.ChannelID, reaction.MessageID, suggestionMessage)
		if err != nil {
			fmt.Println("edit message error: ", err)
		}

		delete(idolSuggestions, suggestion.messageID)
	}
}
