package upload

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/google/uuid"

	"github.com/labstack/echo/v4"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/go-redis/redis"
)

type App struct {
	S3    S3Client
	Redis RedisClient
}

func createApp() (*App, error) {
	app := &App{}

	slog.Info("Redis client created")
	redisdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})
	app.Redis = redisdb

	app.Redis.WrapProcess(func(old func(cmd redis.Cmder) error) func(cmd redis.Cmder) error {
		return func(cmd redis.Cmder) error {
			slog.Info("starting processing: <%s>\n", cmd)
			err := old(cmd)
			slog.Info("finished processing: <%s>\n", cmd)
			return err
		}

	})

	s3client, err := NewS3()

	if err != nil {
		return nil, err
	}

	app.S3 = s3client

	return app, nil
}

func (*App) Run() {

	// err := godotenv.Load()
	//
	// if err != nil {
	// 	slog.Error("Failed to load .env file")
	// 	return
	// }
	e := echo.New()

	// app, err := createApp()
	// if err != nil {
	// 	slog.Error("Failed to create app: %v", err)
	// 	return
	// }
	e.GET("/health_check", func(c echo.Context) error {
		content := struct {
			Status string `json:"status"`
		}{
			Status: "ok",
		}
		return c.JSON(http.StatusOK, content)
	})
	// e.POST("/deploy", app.deploy)

	e.Logger.Fatal(e.Start(":8080"))

}

type Repo struct {
	URL string `json:"repo"`
}

func (a *App) deploy(c echo.Context) error {

	repo := new(Repo)

	if err := c.Bind(repo); err != nil {
		return err
	}

	ch := make(chan string)
	var wg sync.WaitGroup

	id, err := generateId()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Error generating id")
	}

	outputPath := "out/" + id
	err = clone(repo.URL, outputPath)
	slog.Info("Cloning repo" + repo.URL)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Error cloning repo")
	}

	localRepoDir := outputPath
	files, err := getFiles(localRepoDir)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Error getting files")
	}

	for _, file := range files {
		wg.Add(1)
		go uploadFile(file, "gitbit", a.S3, ch, &wg)

		if err != nil {
			return c.JSON(http.StatusInternalServerError, "Error uploading file")
		}
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	res := struct {
		Id string `json:"id"`
	}{Id: id}

	//Enqueue the id of the repo to be built in the other service
	a.Redis.LPush("build-queue", id)
	//Set the status of the repo to uploaded
	a.Redis.HSet(id, "status", "uploaded")

	slog.Info("Deployed repo " + repo.URL + " with id " + id)
	return c.JSON(http.StatusOK, res)
}

func generateId() (string, error) {

	id, err := uuid.NewRandom()
	//grab the first 5 digits of the uuid
	//and return it as a string

	return id.String()[:5], err
}

func clone(repoURL string, outputPath string) error {

	_, err := git.PlainClone(outputPath, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: os.Stdout,
	})

	return err
}

func getFiles(localRepoDir string) ([]string, error) {

	files := []string{}

	err := filepath.Walk(localRepoDir, func(path string, info os.FileInfo, err error) error {
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

func uploadFile(fileName string, bucketName string, client S3Client, ch chan<- string, wg *sync.WaitGroup) error {
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

func NewS3() (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("sa-east-1"))

	client := s3.NewFromConfig(cfg)

	return client, err
}
