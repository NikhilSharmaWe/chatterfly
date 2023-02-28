package main

import (
	"log"
	"net/http"
	"os"

	"github.com/NikhilSharmaWe/chatterfly/controller"
	"github.com/NikhilSharmaWe/chatterfly/router"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load("vars.env")
	if err != nil {
		log.Println("Error loading .env file")
	}
	r := mux.NewRouter()
	router.RegisterRoutes(r)
	go controller.HandleMessages() // I have added it here, since it needs to be run only once

	port := os.Getenv("PORT")
	log.Println("Server starting at localhost: " + port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Println(err)
	}
}
