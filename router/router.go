package router

import (
	"net/http"

	"github.com/NikhilSharmaWe/chatterfly/controller"
	"github.com/gorilla/mux"
)

var RegisterRoutes = func(r *mux.Router) {
	r.Handle("/", http.FileServer(http.Dir("./public/home")))
	r.Handle("/favicon.ico", http.NotFoundHandler())
	r.HandleFunc("/login", controller.Login)
	r.HandleFunc("/signup", controller.Signup)
	r.HandleFunc("/chatroom", controller.Chat)
	r.HandleFunc("/chatroom/{crKey}", controller.ChatRoom)
	r.HandleFunc("/logout", controller.Logout)
	r.HandleFunc("/websocket", controller.HandleConnections)
}
