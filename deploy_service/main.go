package main

import (
	"fmt"
	"io"
	"os"

	"github.com/go-redis/redis"
	"github.com/joho/godotenv"

	"context"
	"path/filepath"

	// "github.com/aws/aws-sdk-go-v2/aws"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type DeployApp struct {
	RedisClient *redis.Client
	S3Client    *s3.Client
}

func (app *DeployApp) downloadS3Folder(bucket string, folder string) error {

	// List all objects in the folder
	listObjectsOutput, err := app.S3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: &bucket,
		Prefix: &folder,
	})

	if err != nil {
		slog.Error("Error listing objects", err)
		return err
	}

	for _, item := range listObjectsOutput.Contents {

		key := *item.Key

		fileDir := filepath.Join(".", filepath.Dir(key))
		if err := os.MkdirAll(fileDir, os.ModePerm); err != nil {
			slog.Error("Error creating dir", fileDir, err)
			continue
		}

		file, err := os.Create(filepath.Join(".", key))

		if err != nil {
			slog.Error("Error creating file", key, err)
			continue
		}

		defer file.Close()

		object, err := app.S3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: &bucket,
			Key:    item.Key,
		})

		if err != nil {
			slog.Error("Error getting object", key, err)
			return err
		}

		defer object.Body.Close()

		if _, err := io.Copy(file, object.Body); err != nil {
			slog.Error("Error downloading object ", key, err)
			continue
		}

		slog.Info("Downloaded object" + key)

	}

	return nil

}
func main() {

	rClient := redis.NewClient(&redis.Options{
		Addr: ":6379",
	})

	err := godotenv.Load()

	if err != nil {
		fmt.Println("Error loading .env file")
		os.Exit(1)
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("sa-east-1"))

	if err != nil {
		fmt.Println("Error loading AWS config")
		os.Exit(1)
	}

	s3Client := s3.NewFromConfig(cfg)

	app := DeployApp{
		RedisClient: rClient,
		S3Client:    s3Client,
	}

	fmt.Println("Hello, World!")

	app.downloadS3Folder("gitbit", "out/eed64")

	// Polls values from a redis queue
	// for {
	//
	// 	fmt.Println("Polling...")
	// 	res := app.RedisClient.BRPop(0, "build-queue").Val()
	// 	fmt.Println(res)
	// }
}
