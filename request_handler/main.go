package main

import (
	"fmt"
	// "io"
	"log"
	"net/http"
	// "path"
	// "sync"

	"strings"

	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	//    "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type RequestApp struct {
	S3Client   *s3.Client
	bucketName string
}

func main() {
	err := godotenv.Load()

	if err != nil {
		slog.Error("Error loading .env file")
		os.Exit(1)
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("sa-east-1"))

	if err != nil {
		slog.Error(fmt.Sprintf("Error loading aws client from config %v", err))
		os.Exit(1)
	}

	app := &RequestApp{
		bucketName: "gitbit",
		S3Client:   s3.NewFromConfig(cfg),
	}

	http.HandleFunc("/*", func(w http.ResponseWriter, r *http.Request) {

		host := r.Host

		//string split
		s := strings.Split(host, ".")[0]

		// //download s3 contents
		targetPath := r.URL.Path

		//
		filePath := filepath.Join("dist", s, targetPath)
		content, err := app.S3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: &app.bucketName,
			Key:    &filePath,
		})

		if err != nil {
			slog.Error("Error getting object", filePath, err)
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("404 Not Found"))
			return
		}
		//
		slog.Info(fmt.Sprintf("Downloading object %v...", filePath))
		//
		var fileType string

		if strings.HasSuffix(targetPath, "html") {
			fileType = "text/html"
		} else if strings.HasSuffix(targetPath, "css") {
			fileType = "text/css"
		} else {
			fileType = "application/javascript"
		}

		// //write response
		w.Header().Set("Content-Type", fileType)
		w.WriteHeader(http.StatusOK)

		var body []byte = make([]byte, *content.ContentLength)
		_, err = content.Body.Read(body)

		if err != nil {
			slog.Error("Error reading object", filePath, err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 Internal Server Error"))
			return
		}
		w.Write(body)
		content.Body.Close()
	})
	log.Printf(">> Listening on port 8080 <<")

	http.ListenAndServe(":8080", nil)
}

// func (app *RequestApp) downloadS3Contents(bucket string, folder string) error {
//
// 	keyChan := make(chan string)
// 	doneChan := make(chan bool)
// 	wg := sync.WaitGroup{}
//
// 	numWorkers := 5
// 	for i := 0; i <= numWorkers; i++ {
// 		wg.Add(1)
// 		go app.worker(context.TODO(), keyChan, &wg)
// 	}
//
// 	go func() {
// 		// List all objects in the folder
// 		listObjectsOutput, err := app.S3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
// 			Bucket: &bucket,
// 			Prefix: &folder,
// 		})
//
// 		if err != nil {
// 			slog.Error(fmt.Sprintf("Error listing objects %v", err))
// 		}
// 		for _, listObject := range listObjectsOutput.Contents {
// 			keyChan <- *listObject.Key
// 		}
//
// 		close(keyChan)
// 	}()
//
// 	go func() {
// 		wg.Wait()
// 		close(doneChan)
// 	}()
//
// 	<-doneChan
// 	return nil
// }
// func (app *RequestApp) worker(ctx context.Context, keysChan <-chan string, wg *sync.WaitGroup) error {
// 	defer wg.Done()
//
// 	for key := range keysChan {
// 		fileDir := filepath.Join(".", filepath.Dir(key))
// 		if err := os.MkdirAll(fileDir, os.ModePerm); err != nil {
// 			slog.Error("Error creating dir", fileDir, err)
// 			continue
// 		}
//
// 		file, err := os.Create(filepath.Join(".", key))
//
// 		if err != nil {
// 			slog.Error("Error creating file", key, err)
// 			continue
// 		}
//
// 		object, err := app.S3Client.GetObject(context.TODO(), &s3.GetObjectInput{
// 			Bucket: &app.bucketName,
// 			Key:    &key,
// 		})
//
// 		if err != nil {
// 			slog.Error("Error getting object", key, err)
// 			return err
// 		}
//
// 		if _, err := io.Copy(file, object.Body); err != nil {
// 			slog.Error("Error downloading object ", key, err)
// 			continue
// 		}
//
// 		/// need to manually call at the end of scope
// 		/// because defer keyword has different effect in goroutines
// 		/// it defers the call to when the channel is closed
// 		file.Close()
// 		object.Body.Close()
// 		slog.Info(fmt.Sprintf("Downloading object %v...", key))
// 	}
// 	return nil
// }
