package main

import (
	"log"
	"net/http"
	"os"

	"github.com/NikhilSharmaWe/chatapp/controller"
	"github.com/NikhilSharmaWe/chatapp/model"
	"github.com/joho/godotenv"
)

func main() {
	model.OpenRedis()

	err := godotenv.Load("vars.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")
	http.Handle("/", http.FileServer(http.Dir("./public/signup")))
	http.Handle("/chatbox/", http.StripPrefix("/chatbox", http.FileServer(http.Dir("./public/chatbox"))))
	http.HandleFunc("/websocket", controller.HandleConnections)
	log.Print("Server starting at localhost:4444")
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
