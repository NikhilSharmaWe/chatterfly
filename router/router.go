package router

import (
	"net/http"

	"github.com/NikhilSharmaWe/chatterfly/controller"
	"github.com/gorilla/mux"
)

var RegisterRoutes = func(r *mux.Router) {
	r.Handle("/", http.FileServer(http.Dir("./public/home")))
	r.Handle("/favicon.ico", http.NotFoundHandler())
	r.HandleFunc("/login", controller.MakeHTTPHandlerFunc(controller.HandleLogin))
	r.HandleFunc("/login/", controller.MakeHTTPHandlerFunc(controller.HandleLogin))
	r.HandleFunc("/signup", controller.MakeHTTPHandlerFunc(controller.HandleSignup))
	r.HandleFunc("/signup/", controller.MakeHTTPHandlerFunc(controller.HandleSignup))
	r.HandleFunc("/logout", controller.MakeHTTPHandlerFunc(controller.HandleLogout))
	r.HandleFunc("/logout/", controller.MakeHTTPHandlerFunc(controller.HandleLogout))
	r.PathPrefix("/chatroom/c/{crKey}/").HandlerFunc(controller.MakeHTTPHandlerFunc((controller.HandleChatroom)))
	r.HandleFunc("/chatroom/c/{crKey}", controller.PathWithoutFS)
	r.PathPrefix("/chatroom/").HandlerFunc(controller.MakeHTTPHandlerFunc(controller.HandleCreateChatroom))
	r.HandleFunc("/chatroom", controller.PathWithoutFS)
	r.HandleFunc("/websocket", controller.MakeHTTPHandlerFunc(controller.HandleConnections))
	r.HandleFunc("/linkdata", controller.MakeHTTPHandlerFunc(controller.SendUserData))
}
