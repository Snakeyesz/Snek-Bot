package utils

import (
	"github.com/bwmarrin/discordgo"

	"github.com/Snakeyesz/snek-bot/cache"
)

// Will panic if error is not nil
func PanicCheck(err error) {
	if err != nil {
		panic(err)
	}
}

// alert user of custom error if any exists
func AlertUserOfError(err error, session *discordgo.Session, msg *discordgo.Message) {
	translations := cache.Geti18nTranslations()
	errCode := err.Error()

	// check if the error code has a user translation
	if !translations.ExistsP(errCode) {
		return
	}

	// output translation to user
	translation := translations.Path(errCode).Data().(string)
	session.ChannelMessageSend(msg.ChannelID, translation)
}
