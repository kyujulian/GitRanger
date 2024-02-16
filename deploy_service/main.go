package main

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/go-redis/redis"
	"github.com/joho/godotenv"

	"context"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	numWorkers = 5
)

type DeployApp struct {
	RedisClient *redis.Client
	S3Client    *s3.Client
	bucketName  string
}

func (app *DeployApp) downloadS3Folder(bucket string, folder string) error {

	keyChan := make(chan string)
	doneChan := make(chan bool)
	wg := sync.WaitGroup{}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go app.worker(context.TODO(), keyChan, &wg)
	}

	go func() {

		// List all objects in the folder
		listObjectsOutput, err := app.S3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: &bucket,
			Prefix: &folder,
		})

		if err != nil {
			slog.Error(fmt.Sprintf("Error listing objects %v", err))
		}
		for _, object := range listObjectsOutput.Contents {
			keyChan <- *object.Key
		}
		close(keyChan)

	}()

	go func() {
		wg.Wait()
		close(doneChan)
	}()

	<-doneChan
	return nil

}

func (app *DeployApp) worker(ctx context.Context, keysChan <-chan string, wg *sync.WaitGroup) error {
	defer wg.Done()

	for key := range keysChan {

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

		object, err := app.S3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: &app.bucketName,
			Key:    &key,
		})

		if err != nil {
			slog.Error("Error getting object", key, err)
			return err
		}

		if _, err := io.Copy(file, object.Body); err != nil {
			slog.Error("Error downloading object ", key, err)
			continue
		}

		/// need to manually call at the end of scope
		/// because defer keyword has different effect in goroutines
		/// it defers the call to when the channel is closed
		file.Close()
		object.Body.Close()
		slog.Info(fmt.Sprintf("Downloading object %v...", key))
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
		bucketName:  "gitbit",
	}

	fmt.Println("Hello, World!")

	//Polls values from a redis queue
	for {

		fmt.Println("Polling...")
		res := app.RedisClient.BRPop(0, "build-queue").Val()
		fmt.Println(res[0])
		app.downloadS3Folder("gitbit", filepath.Join("out", res[0])) // how will I get the proper filepath?
	}
}

func getFiles(id string) ([]string, error) {
	files := []string{}

	err := filepath.Walk("out/"+id, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// skip directories
		if info.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})

	return files, err
}

func uploadFile(fileName string, bucketName string, client *s3.Client, ch chan<- string, wg *sync.WaitGroup) error {
	fileContent, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Error opening file")
		return err
	}

	defer wg.Done()
	defer fileContent.Close()

	result, err := client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileName),
		Body:   fileContent,
	})

	if err != nil {
		slog.Error("failed to upload file %s to bucket %s: %w", fileName, "gitbit", err)
		return err

	}
	fmt.Println(result)

	ch <- fileName
	return nil
}
