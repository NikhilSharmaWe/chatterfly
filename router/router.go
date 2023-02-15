package router

import (
	"net/http"

	"github.com/NikhilSharmaWe/chatapp/controller"
	"github.com/gorilla/mux"
)

var RegisterRoutes = func(r *mux.Router) {
	r.Handle("/", http.FileServer(http.Dir("./public")))
	r.HandleFunc("/websocket", controller.HandleConnections)
}
