package model

import (
	"log"
	"os"

	"github.com/go-redis/redis"
	"github.com/joho/godotenv"
)

type User struct {
	Username  string `json:"username"`
	Firstname string `json:"firstname"`
	Lastname  string `json:"lastname"`
	Password  []byte `json:"password"`
}

type Session struct {
	SessionId string `json:"sid"`
	Username  string `json:"username"`
}

type Chat struct {
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Message  string `json:"message"`
}

func OpenRedis() *redis.Client {
	err := godotenv.Load("vars.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	url := os.Getenv("REDIS_URL")
	opts, err := redis.ParseURL(url)
	if err != nil {
		panic("Unable to parse url")
	}

	rdb := redis.NewClient(opts)
	return rdb
}
