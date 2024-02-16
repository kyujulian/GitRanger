package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"context"

	"github.com/go-git/go-git/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/joho/godotenv"
	"log"

	"github.com/go-redis/redis"

	"log/slog"
)

type App struct {
	Client *s3.Client
	Db     *redis.Client
}

func main() {

	slog.Info("Starting gitbit")
	redisdb := redis.NewClient(&redis.Options{
		Addr: ":6379",
	})

	slog.Info("Redis client created")
	redisdb.WrapProcess(func(old func(cmd redis.Cmder) error) func(cmd redis.Cmder) error {
		return func(cmd redis.Cmder) error {
			fmt.Printf("starting processing: <%s>\n", cmd)
			err := old(cmd)
			fmt.Printf("finished processing: <%s>\n", cmd)
			return err
		}
	})

	err := godotenv.Load()

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Hello, World!")

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("sa-east-1"))

	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	s3client := s3.NewFromConfig(cfg)

	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})

	app := &App{Client: s3client, Db: redisdb}

	e.POST("/deploy", app.deploy)

	e.GET("/status/:id", func(c echo.Context) error {

		id := c.Param("id")
		res, err := app.Db.HGet(id, "status").Result()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, "Error getting status")
		}
		return c.JSON(http.StatusOK, res)

	})

	e.Logger.Fatal(e.Start(":1323"))
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

	slog.Info("Cloning repo")
	err = clone(repo.URL, id)
	slog.Info("Cloning repo" + repo.URL)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Error cloning repo")
	}

	files, err := getFiles(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Error getting files")
	}

	for _, file := range files {
		wg.Add(1)
		go uploadFile(file, "gitbit", a.Client, ch, &wg)

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
	a.Db.LPush("build-queue", id)
	//Set the status of the repo to uploaded
	a.Db.HSet(id, "status", "uploaded")

	slog.Info("Deployed repo " + repo.URL + " with id " + id)
	return c.JSON(http.StatusOK, res)
}

func generateId() (string, error) {

	id, err := uuid.NewRandom()
	//grab the first 5 digits of the uuid
	//and return it as a string

	return id.String()[:5], err
}

func clone(repoURL string, id string) error {

	_, err := git.PlainClone("./out/"+id, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: os.Stdout,
	})

	return err
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
