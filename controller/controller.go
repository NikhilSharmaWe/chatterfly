package controller

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/NikhilSharmaWe/chatapp/model"
	"github.com/gorilla/websocket"
)

var (
	rdb         = model.RDB
	clients     = make(map[*websocket.Conn]bool)
	broadcaster = make(chan model.Chat)
	upgrader    = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func HandleConnections(w http.ResponseWriter, r *http.Request) {
	var cb *model.ChatBox
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		panic(err)
	}

	defer ws.Close()

	err = json.NewDecoder(r.Body).Decode(cb)
	if err != nil {
		log.Fatal("Unable to decode chatbox info")
		w.WriteHeader(http.StatusInternalServerError)
	}

	go handleMessages(ws, cb)

	chatboxExists, correctPassword := chatBoxExistsAndPassword(cb, w, r)

	if chatboxExists {
		if correctPassword {
			if rdb.Exists("chat").Val() != 0 {
				sendPreviousChats(ws, cb)
			}
		} else {
			// make the user know that for these 2 friends chatbox, password is wrong
			// may be we can redirect them to a error message and return the function
		}
	}

	for {
		var chat model.Chat
		if err != ws.ReadJSON(&chat) {
			panic(err)
		}
		broadcaster <- chat
	}
}

func chatBoxExistsAndPassword(cb *model.ChatBox, w http.ResponseWriter, r *http.Request) (bool, bool) {
	var chatboxExists bool
	var correctPassword bool

	err := json.NewDecoder(r.Body).Decode(cb)
	if err != nil {
		log.Fatal("Unable to decode chatbox info")
		w.WriteHeader(http.StatusInternalServerError)
	}

	chatboxes, err := rdb.LRange("chatbox", 0, -1).Result()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)

		panic(err)
	}

	var existingChatBox *model.ChatBox

	for _, chatbox := range chatboxes {
		json.Unmarshal([]byte(chatbox), existingChatBox)
		if (existingChatBox.User == cb.User && existingChatBox.Friend == cb.Friend) || (existingChatBox.User == cb.Friend && existingChatBox.Friend == cb.User) {
			chatboxExists = true
			if existingChatBox.Password == cb.Password {
				correctPassword = true
			}
		}
	}

	return chatboxExists, correctPassword
}

func sendPreviousChats(ws *websocket.Conn, cb *model.ChatBox) {
	chats, err := rdb.LRange("chat", 0, -1).Result()
	if err != nil {
		panic(err)
	}

	// send previous messages
	for _, chat := range chats {
		var chatContent model.Chat
		json.Unmarshal([]byte(chat), &chatContent)
		err := messageClient(ws, chatContent, cb)
		if err != nil {
			panic(err)
		}
	}
}

func messageClient(ws *websocket.Conn, chat model.Chat, cb *model.ChatBox) error {
	if cb.User == chat.Sender && cb.Friend == chat.Receiver {
		err := ws.WriteJSON(chat)
		if err != nil && unsafeError(err) {
			log.Printf("error: %v", err)
			ws.Close()
			delete(clients, ws)
			return err
		}
	}
	return nil
}

func storeInRedis(chat model.Chat) {
	json, err := json.Marshal(chat)
	if err != nil {
		panic(err)
	}
	if err = rdb.RPush("chat", json).Err(); err != nil {
		panic(err)
	}
}

func handleMessages(ws *websocket.Conn, cb *model.ChatBox) {
	for {
		chat := <-broadcaster
		storeInRedis(chat)
		err := messageClient(ws, chat, cb)
		if err != nil {
			panic(err)
		}
	}
}

func unsafeError(err error) bool {
	return !websocket.IsCloseError(err, websocket.CloseGoingAway) && err != io.EOF
}
