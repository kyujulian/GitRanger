package upload

import (
	"fmt"
	"github.com/spf13/cobra"
	"log/slog"
)

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload a file to S3",
	Long:  `Upload a file to S3`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here

		app, err := createApp().defaultRedisClient().defaultS3Client()

		if err != nil {
			slog.Error("Failed to create app %v", err)
			return
		}
		app.Run()

		// fmt.Println("Uploading file to S3")
	},
}

func Execute() {
	if err := uploadCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}
