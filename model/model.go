package model

import (
	"log"
	"os"

	"github.com/go-redis/redis"
	"github.com/joho/godotenv"
)

var (
	RDB *redis.Client
)

type ChatBox struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Friend   string `json:"friend"`
}

type Chat struct {
	Sender   string `json:"sender"`
	Receiver string `json:"receiver"`
	Message  string `json:"message"`
}

func OpenRedis() {
	err := godotenv.Load("vars.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	url := os.Getenv("REDIS_URL")
	opts, err := redis.ParseURL(url)
	if err != nil {
		panic("Unable to parse url")
	}

	RDB = redis.NewClient(opts)
}
