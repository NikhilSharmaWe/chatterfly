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
	r.HandleFunc("/login/", controller.Login)
	r.HandleFunc("/signup", controller.Signup)
	r.HandleFunc("/signup/", controller.Signup)
	r.HandleFunc("/logout", controller.Logout)
	r.HandleFunc("/logout/", controller.Logout)
	r.PathPrefix("/chatroom/{crKey}/").HandlerFunc(controller.ChatRoom)
	r.HandleFunc("/chatroom/{crKey}", controller.PathWithoutFS)
	r.PathPrefix("/chatroom/").HandlerFunc(controller.Chat)
	r.HandleFunc("/chatroom", controller.PathWithoutFS)
	r.HandleFunc("/websocket", controller.HandleConnections)
	r.HandleFunc("/linkdata", controller.SendUserData)
}
