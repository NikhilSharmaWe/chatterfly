package main

import (
	"log"
	"net/http"
	"os"

	"github.com/NikhilSharmaWe/chatterfly/controller"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load("vars.env")
	if err != nil {
		log.Println("Error loading .env file")
	}

	port := os.Getenv("PORT")
	http.Handle("/", http.FileServer(http.Dir("./public/home")))
	http.Handle("/favicon.ico", http.NotFoundHandler())
	http.HandleFunc("/login", controller.Login)
	http.HandleFunc("/signup", controller.Signup)
	http.HandleFunc("/chat", controller.Chat)
	http.HandleFunc("/logout", controller.Logout)

	log.Println("Server starting at localhost: " + port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Println(err)
	}
}
