package components

import (
	"context"
	"fmt"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"

	"github.com/Snakeyesz/snek-bot/cache"
	"github.com/Snakeyesz/snek-bot/utils"
)

// initialize and set up discord bot
func InitGoogleDrive() {
	fmt.Println("Initializing google drive...")
	ctx := context.Background()

	// Get app configs
	appConfigs := cache.GetAppConfig()

	// load drive configs
	driveConfigs := appConfigs.Path("google_drive").Bytes()
	config, err := google.JWTConfigFromJSON(driveConfigs, drive.DriveScope)
	utils.PanicCheck(err)

	// create drive service
	client := config.Client(ctx)
	driveService, err := drive.New(client)
	utils.PanicCheck(err)

	cache.SetGoogleDriveService(driveService)
}
