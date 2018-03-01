package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/Snakeyesz/snek-bot/components"
	"github.com/Snakeyesz/snek-bot/utils"
)

// Bot Entry Point
func main() {

	// Initialize and loadcomponents
	components.LoadAppConfig()
	components.Loadi18nTranslations()
	components.InitGoogleDrive()
	components.ConnectMongoDB()
	components.InitDiscordBot() // always load last

	// connect discord bot
	discord := cache.GetDiscordSession()
	err := discord.Open()
	utils.PanicCheck(err)

	// Run bot until connection is closed or interupted
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	botRuntimeCh := make(chan os.Signal, 1)
	signal.Notify(botRuntimeCh, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)

	// close bot when signal to close is recieved in the botRuntime channel
	<-botRuntimeCh
	fmt.Println("Bot is now closeing.")
	discord.Close()
}
