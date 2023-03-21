package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	clients            = make(map[*websocket.Conn]model.ClientInfo)
	broadcaster        = make(chan model.Chat)
	upgrader           = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func init() {
	rdb = model.OpenRedis()
	userCollection = model.CreateMongoCollection(ctx, "user-data")
	chatroomCollection = model.CreateMongoCollection(ctx, "chat-room")
	chatCollection = model.CreateMongoCollection(ctx, "chat")
}

func Signup(w http.ResponseWriter, r *http.Request) {
	if alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/chatroom/", http.StatusSeeOther)
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
			Username:  un,
			Firstname: fn,
			Lastname:  ln,
		}
		err := storeInRedis(sId, w, session)
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

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
		err = storeInMongo(userCollection, &user)
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:  "chatterfly-cookie",
			Value: sId,
		})
		http.Redirect(w, r, "/chatroom/", http.StatusSeeOther)
		return
	}
	http.ServeFile(w, r, "./public/signup/index.html")
}

func Login(w http.ResponseWriter, r *http.Request) {
	if alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/chatroom/", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodPost {
		un := r.PostFormValue("username")
		pw := r.PostFormValue("password")
		user := getUser(w, un)

		err := bcrypt.CompareHashAndPassword(user.Password, []byte(pw))
		if err != nil {
			log.Println("Username and/or password do not match")
			http.Error(w, "Username and/or password do not match", http.StatusForbidden)
			return
		}
		sId := "session-" + uuid.NewV4().String()
		session := model.Session{
			Username:  un,
			Firstname: user.Firstname,
			Lastname:  user.Lastname,
		}
		err = storeInRedis(sId, w, session)
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:  "chatterfly-cookie",
			Value: sId,
		})

		http.Redirect(w, r, "/chatroom/", http.StatusSeeOther)
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
		err := storeInMongo(chatroomCollection, &cr)
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		var session model.Session
		cookie, _ := r.Cookie("chatterfly-cookie")
		sId := cookie.Value

		getSession(sId, &session)
		un := session.Username
		user := getUser(w, un)
		crs := append(user.Chatrooms, cr)
		updateCRListForUser(user.Username, crs)

		http.Redirect(w, r, fmt.Sprintf("/chatroom/%v/", crKey), http.StatusSeeOther)
		return
	}
	http.StripPrefix("/chatroom", http.FileServer(http.Dir("./public/chat"))).ServeHTTP(w, r)
}

func ChatRoom(w http.ResponseWriter, r *http.Request) {
	if !alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	var session model.Session
	cookie, _ := r.Cookie("chatterfly-cookie")
	sId := cookie.Value
	err := getSession(sId, &session)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	params := mux.Vars(r)
	crKey := params["crKey"]
	_, err = getChatRoom(w, crKey)
	if err != nil {
		log.Println(err)
		if err == mongo.ErrNoDocuments {
			http.Error(w, "Chatroom does not exists", http.StatusInternalServerError)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	session.ChatRoomKey = crKey
	// update the session with the chatroomkey
	err = storeInRedis(sId, w, session)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	http.StripPrefix("/chatroom/"+crKey, http.FileServer(http.Dir("./public/chatroom"))).ServeHTTP(w, r)
}

func PathWithoutFS(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	http.Redirect(w, r, path+"/", http.StatusSeeOther)
}

func HandleConnections(w http.ResponseWriter, r *http.Request) {
	if !alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	var session model.Session
	cookie, _ := r.Cookie("chatterfly-cookie")
	sId := cookie.Value

	getSession(sId, &session)
	un := session.Username
	fn := session.Firstname
	crKey := session.ChatRoomKey

	chat := model.Chat{}
	filter := bson.M{"key": crKey}
	cr, _ := getChatRoom(w, crKey)
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	defer ws.Close()
	clients[ws] = model.ClientInfo{
		Key:       crKey,
		Username:  un,
		Firstname: fn,
	}

	err = chatCollection.FindOne(context.Background(), filter).Decode(&chat)
	if err == nil {
		err := sendOldChats(crKey, ws)
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	messageClient(ws, cr) // this is sending chatroom for displaying the chatroomname on top of chatroom

	user := getUser(w, un)
	var userAlreadyMember bool
	for _, chatRoom := range user.Chatrooms {
		if chatRoom.Key == crKey {
			userAlreadyMember = true
			break
		}
	}
	if !userAlreadyMember {
		crs := append(user.Chatrooms, cr)
		updateCRListForUser(user.Username, crs)
	}

	for {
		var msg model.Chat
		err := ws.ReadJSON(&msg)
		msg.Username = clients[ws].Username
		msg.Firstname = clients[ws].Firstname
		if err != nil {
			delete(clients, ws)
			break
		}
		broadcaster <- msg
	}
}

func SendUserData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var session model.Session
	cookie, _ := r.Cookie("chatterfly-cookie")
	sId := cookie.Value

	getSession(sId, &session)
	un := session.Username
	user := getUser(w, un)
	err := json.NewEncoder(w).Encode(user)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
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

func storeInRedis(key string, w http.ResponseWriter, value interface{}) error {
	json, err := json.Marshal(value)
	if err != nil {
		log.Println(err)
		return err
	}
	if err = rdb.Set(key, json, 0).Err(); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func deleteInRedis(key string, w http.ResponseWriter) {
	_, err := rdb.Del(key).Result()
	if err != nil {
		log.Println(err)
		http.Error(w, "Unable to delete data", http.StatusInternalServerError)
	}
}

func getSession(key string, obj *model.Session) error {
	jsonObj, err := rdb.Get(key).Result()
	if err != nil {
		return err
	}
	err = json.Unmarshal([]byte(jsonObj), &obj)

	if err != nil {
		return err
	}
	return nil
}

func storeInMongo(collection *mongo.Collection, value interface{}) error {
	_, err := collection.InsertOne(ctx, value)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func getUser(w http.ResponseWriter, un string) model.User {
	user := model.User{}
	filter := bson.M{"username": un}
	userCollection.FindOne(context.Background(), filter).Decode(&user)
	return user
}

func getChatRoom(w http.ResponseWriter, key string) (model.ChatRoom, error) {
	chatRoom := model.ChatRoom{}
	filter := bson.M{"key": key}
	err := chatroomCollection.FindOne(context.Background(), filter).Decode(&chatRoom)
	if err != nil {
		return chatRoom, err
	}
	return chatRoom, nil
}

func getChats(crKey string) ([]*model.Chat, error) {
	var chats []*model.Chat
	filter := bson.M{"key": crKey}
	cur, err := chatCollection.Find(ctx, filter)
	if err != nil {
		return chats, err
	}
	for cur.Next(ctx) {
		var chat model.Chat
		err := cur.Decode(&chat)
		if err != nil {
			return chats, err
		}
		chats = append(chats, &chat)
	}
	if err := cur.Err(); err != nil {
		return chats, nil
	}
	cur.Close(ctx)

	if len(chats) == 0 {
		return chats, mongo.ErrNoDocuments
	}

	return chats, nil
}

func updateCRListForUser(un string, updatedList []model.ChatRoom) error {
	filter := bson.D{primitive.E{Key: "username", Value: un}}
	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "chatrooms", Value: updatedList},
	}}}

	u := model.User{}
	return userCollection.FindOneAndUpdate(ctx, filter, update).Decode(&u)
}

func sendOldChats(crKey string, ws *websocket.Conn) error {
	chats, err := getChats(crKey)
	if err != nil {
		log.Println(err)
		return err
	}

	for _, chat := range chats {
		if chat.Key == clients[ws].Key {
			err := messageClient(ws, *chat)
			if err != nil {
				return nil
			}
		}
	}
	return nil
}

func messageClients(msg model.Chat) error {
	crKey := msg.Key
	for client := range clients {
		if crKey == clients[client].Key {
			err := messageClient(client, msg)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func messageClient(ws *websocket.Conn, msg interface{}) error {
	err := ws.WriteJSON(msg)
	if err != nil && unsafeError(err) {
		log.Println(err)
		ws.Close()
		delete(clients, ws)
		return err
	}
	return nil
}

func HandleMessages() {
	for {
		msg := <-broadcaster
		storeInMongo(chatCollection, msg)
		messageClients(msg)
	}
}

func unsafeError(err error) bool {
	return !websocket.IsCloseError(err, websocket.CloseGoingAway) && err != io.EOF
}
