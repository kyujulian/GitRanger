package main

import (
	"fmt"
	"github.com/go-redis/redis"
	"os"

	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type DeployApp struct {
	RedisClient *redis.Client
	S3Client    *s3.Client
}

func main() {

	dbClient := redis.NewClient(&redis.Options{
		Addr: ":6379",
	})

	bucket := os.Getenv("S3_BUCKET")
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("sa-east-1"))

	s3Client := s3.NewFromConfig(cfg)

	app := DeployApp{
		RedisClient: dbClient,
		S3Client:    s3Client,
	}

	fmt.Println("Hello, World!")

	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Polls values from a redis queue
	for {

		fmt.Println("Polling...")
		res := app.RedisClient.BRPop(0, "build-queue").Val()
		fmt.Println(res)
	}
}
