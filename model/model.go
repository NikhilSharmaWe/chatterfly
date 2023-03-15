package model

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	CreatedAt time.Time          `bson:"created_at"`
	Username  string             `bson:"username"`
	Firstname string             `bson:"firstname"`
	Lastname  string             `bson:"lastname"`
	Password  []byte             `bson:"password"`
	Chatrooms []ChatRoom         `bson:"chatrooms"`
}

type Session struct {
	Username    string `json:"username"`
	Firstname   string `json:"firstname"`
	Lastname    string `json:"lastname"`
	ChatRoomKey string `json:"key"`
}

type ChatRoom struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	CreatedAt    time.Time          `bson:"created_at"`
	Key          string             `bson:"key"`
	ChatRoomName string             `bson:"chatroomname"`
}

type Chat struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	CreatedAt time.Time          `bson:"created_at"`
	Username  string             `bson:"username"`
	Firstname string             `bson:"firstname"`
	Key       string             `bson:"key"`
	Message   string             `bson:"message"`
}

type ClientInfo struct {
	Key       string
	Username  string
	Firstname string
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

func CreateMongoCollection(ctx context.Context, name string) *mongo.Collection {
	err := godotenv.Load("vars.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	url := os.Getenv("MONGO_URL")
	clientOptions := options.Client().ApplyURI(url)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	collection := client.Database(name).Collection("users")
	return collection
}
