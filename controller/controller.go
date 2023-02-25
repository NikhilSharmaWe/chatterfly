package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/NikhilSharmaWe/chatterfly/model"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

var (
	rdb                *redis.Client
	userCollection     *mongo.Collection
	chatroomCollection *mongo.Collection
	chatCollection     *mongo.Collection
	ctx                = context.Background()
	clients            = make(map[*websocket.Conn]bool)
	broadcaster        = make(chan model.Chat)
	upgrader           = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type Connection struct {
	ChatRoomKey string
	Conn        *websocket.Conn
}

// type server struct {
// 	chatroom string
// 	conns    map[*websocket.Conn]bool
// }

// func newServer(chatroom string) *server {
// 	return &server{
// 		chatroom: chatroom,
// 		conns:    make(map[*websocket.Conn]bool),
// 	}
// }

// func (s *server) handleWS(ws *websocket.Conn) {

// }

func HandleConnections(w http.ResponseWriter, r *http.Request) {
	if alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/chatroom", http.StatusSeeOther)
		return
	}
	params := mux.Vars(r)
	crKey := params["crKey"]
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	defer ws.Close()
	clients[ws] = true

	connection := Connection{
		ChatRoomKey: crKey,
		Conn:        ws,
	}

	chat := model.Chat{}
	filter := bson.M{"key": crKey}
	err = chatCollection.FindOne(context.Background(), filter).Decode(&chat)
	if err == nil {
		connection.sendOldChat(w, r)
	}

}

func (conn Connection) sendOldChat(w http.ResponseWriter, r *http.Request) {

}
func init() {
	rdb = model.OpenRedis()
	userCollection = model.CreateMongoCollection(ctx, "user-data")
	chatroomCollection = model.CreateMongoCollection(ctx, "chat-room")
	chatCollection = model.CreateMongoCollection(ctx, "chat")
}

func Signup(w http.ResponseWriter, r *http.Request) {
	if alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/chatroom", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodPost {
		un := r.PostFormValue("username")
		pw := r.PostFormValue("password")
		fn := r.PostFormValue("firstname")
		ln := r.PostFormValue("lastname")

		du := getUser(w, un)
		if du.Username == un {
			log.Println("Username already present")
			http.Error(w, "Username already present", http.StatusForbidden)
			return
		}

		sId := "session-" + uuid.NewV4().String()
		session := model.Session{
			Username: un,
		}
		storeInRedis(sId, w, session)

		bs, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		user := model.User{
			ID:        primitive.NewObjectID(),
			CreatedAt: time.Now(),
			Username:  un,
			Password:  bs,
			Firstname: fn,
			Lastname:  ln,
		}
		// store(un, w, user)
		storeInMongo(w, userCollection, &user)

		http.SetCookie(w, &http.Cookie{
			Name:  "chatterfly-cookie",
			Value: sId,
		})
		http.Redirect(w, r, "/chatroom", http.StatusSeeOther)
		return
	}
	http.ServeFile(w, r, "./public/signup/index.html")
}

func Login(w http.ResponseWriter, r *http.Request) {
	if alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/chatroom", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodPost {
		un := r.PostFormValue("username")
		pw := r.PostFormValue("password")
		user := getUser(w, un)
		fmt.Println(user)

		err := bcrypt.CompareHashAndPassword(user.Password, []byte(pw))
		if err != nil {
			log.Println("Username and/or password do not match")
			http.Error(w, "Username and/or password do not match", http.StatusForbidden)
			return
		}
		sId := "session-" + uuid.NewV4().String()
		session := model.Session{
			Username: un,
		}
		storeInRedis(sId, w, session)

		http.SetCookie(w, &http.Cookie{
			Name:  "chatterfly-cookie",
			Value: sId,
		})

		http.Redirect(w, r, "/chatroom", http.StatusSeeOther)
		return
	}
	http.ServeFile(w, r, "./public/login/index.html")
}

func Logout(w http.ResponseWriter, r *http.Request) {
	if !alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	cookie, _ := r.Cookie("chatterfly-cookie")
	sId := cookie.Value
	deleteInRedis(sId, w)
	cookie = &http.Cookie{
		Name:   "chatterfly-cookie",
		Value:  "",
		MaxAge: -1,
	}

	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func Chat(w http.ResponseWriter, r *http.Request) {
	if !alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodPost {
		name := r.PostFormValue("name")
		crKey := uuid.NewV4().String()
		cr := model.ChatRoom{
			ChatRoomName: name,
			Key:          crKey,
		}
		storeInMongo(w, chatroomCollection, &cr)
		http.Redirect(w, r, fmt.Sprintf("/chatroom/%v", crKey), http.StatusSeeOther)
		return
	}
	http.ServeFile(w, r, "./public/chat/index.html")
}

func ChatRoom(w http.ResponseWriter, r *http.Request) {
	if !alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	params := mux.Vars(r)
	crKey := params["crKey"]
	chatRoom := getChatRoom(w, crKey)
	fmt.Fprintf(w, "ChatRoomName: %v\nChatRoomKey: %v", chatRoom.ChatRoomName, chatRoom.Key)
}

func alreadyLoggedIn(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie("chatterfly-cookie")
	if err == http.ErrNoCookie {
		return false
	}

	sId := cookie.Value
	_, err = rdb.Get(sId).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			log.Println(err)
			http.Error(w, "Entity not found", http.StatusInternalServerError)
		} else {
			log.Println(err)
			http.Error(w, "Unable to get the session info", http.StatusInternalServerError)
		}
		return false
	}
	return true
}

// functions dealing with redis operations
func storeInRedis(key string, w http.ResponseWriter, value interface{}) {
	json, err := json.Marshal(value)
	if err != nil {
		log.Println(err)
		http.Error(w, "Unable to marshal data", http.StatusInternalServerError)
		panic(err)
	}
	if err = rdb.Set(key, json, 0).Err(); err != nil {
		log.Println(err)
		http.Error(w, "Unable to add data", http.StatusInternalServerError)
		panic(err)
	}
}

func deleteInRedis(key string, w http.ResponseWriter) {
	_, err := rdb.Del(key).Result()
	if err != nil {
		log.Println(err)
		http.Error(w, "Unable to delete data", http.StatusInternalServerError)
	}
}

func getInRedis(key string, w http.ResponseWriter, obj interface{}) error {
	jsonObj, err := rdb.Get(key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			log.Println(err)
			http.Error(w, "Entity not found", http.StatusInternalServerError)
			return err
		} else {
			log.Println(err)
			http.Error(w, "Unable to get the obj", http.StatusInternalServerError)
			return err
		}
	}

	err = json.Unmarshal([]byte(jsonObj), obj)
	if err != nil {
		log.Println(err)
		http.Error(w, "Error while unmarshaling obj", http.StatusInternalServerError)
		return err
	}
	return nil
}

// function dealing with mongo operations
func storeInMongo(w http.ResponseWriter, collection *mongo.Collection, value interface{}) {
	_, err := collection.InsertOne(ctx, value)
	if err != nil {
		log.Println(err)
		http.Error(w, "Error while adding new data in mongo", http.StatusInternalServerError)
		panic(err)
	}
}

func getUser(w http.ResponseWriter, un string) model.User {
	user := model.User{}
	filter := bson.M{"username": un}
	err := userCollection.FindOne(context.Background(), filter).Decode(&user)
	if err != nil {
		return user
	}
	return user
}

func getChatRoom(w http.ResponseWriter, key string) model.ChatRoom {
	chatRoom := model.ChatRoom{}
	filter := bson.M{"key": key}
	err := chatroomCollection.FindOne(context.Background(), filter).Decode(&chatRoom)
	if err != nil {
		return chatRoom
	}
	return chatRoom
}

func getChats(w http.ResponseWriter, crKey string) {

}
