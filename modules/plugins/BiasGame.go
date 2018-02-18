package plugins

import (
	"encoding/json"
	"fmt"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/bwmarrin/discordgo"
)

type BiasGame struct{}

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
	driveService := cache.GetGoogleDriveService()

	results, err := driveService.Files.List().Q("mimeType = \"image/gif\" or mimeType = \"image/jpeg\" or mimeType = \"image/png\" or mimeType = \"application/vnd.google-apps.folder\"").Fields("nextPageToken, files(id, name)").Do()

	// r, err := driveService.Files.List().Q(fmt.Sprintf(driveSearchText, driveFolderID)).Fields(googleapi.Field(driveFieldsText)).PageSize(1000).Do()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(json.MarshalIndent(results, "", ""))
	fmt.Println("Files", results.Files)
	fmt.Println("Files:")
	if len(results.Files) > 0 {

		for _, i := range results.Files {
			fmt.Printf("%s (%s)\n", i.Name, i.Id)
		}
	} else {
		fmt.Println("No files found.")
	}

}
