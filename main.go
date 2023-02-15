package main

import (
	"log"
	"net/http"
	"os"

	"github.com/NikhilSharmaWe/chatapp/model"
	"github.com/NikhilSharmaWe/chatapp/router"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	model.OpenRedis()

	err := godotenv.Load("vars.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")
	r := mux.NewRouter()
	router.RegisterRoutes(r)
	log.Print("Server starting at localhost:4444")
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
