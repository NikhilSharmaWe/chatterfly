package controller

import (
	"context"
	"encoding/json"
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

type apiFunc func(w http.ResponseWriter, r *http.Request) error

type apiError struct {
	Error string `json:"error"`
}

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

func HandleSignup(w http.ResponseWriter, r *http.Request) error {
	loggedIn, err := alreadyLoggedIn(r)
	if loggedIn {
		http.Redirect(w, r, "/chatroom/", http.StatusSeeOther)
		return nil
	}

	if err != nil {
		return internalServerError(w, err)
	}

	if r.Method == http.MethodPost {
		user, err := createUserFromForm(w, r)
		if err != nil {
			return internalServerError(w, err)
		}

		_, err = getUser(user.Username)
		if err == nil {
			return permissionDenied(w, fmt.Sprintf("username %s already exists", user.Username))
		}

		err = storeInMongo(userCollection, &user)
		if err != nil {
			return internalServerError(w, err)
		}

		sID := "session-" + uuid.NewV4().String()
		session := createSessionFromUser(user)

		err = storeInRedis(sID, session)
		if err != nil {
			return internalServerError(w, err)
		}

		createCookie(w, sID)
		http.Redirect(w, r, "/chatroom/", http.StatusSeeOther)
		return nil
	}

	http.ServeFile(w, r, "./public/signup/index.html")
	return nil
}

func HandleLogin(w http.ResponseWriter, r *http.Request) error {
	loggedIn, err := alreadyLoggedIn(r)
	if loggedIn {
		http.Redirect(w, r, "/chatroom/", http.StatusSeeOther)
		return nil
	}

	if err != nil {
		return internalServerError(w, err)
	}

	if r.Method == http.MethodPost {
		un := r.PostFormValue("username")
		pw := r.PostFormValue("password")
		user, err := getUser(un)
		if err != nil {
			return fmt.Errorf("user with username %s does not exists", un)
		}

		err = bcrypt.CompareHashAndPassword(user.Password, []byte(pw))
		if err != nil {
			return permissionDenied(w, "username and password do not match")
		}

		sID := "session-" + uuid.NewV4().String()
		session := createSessionFromUser(user)

		err = storeInRedis(sID, session)
		if err != nil {
			return internalServerError(w, err)
		}

		createCookie(w, sID)
		http.Redirect(w, r, "/chatroom/", http.StatusSeeOther)
		return nil
	}

	http.ServeFile(w, r, "./public/login/index.html")
	return nil
}

func HandleLogout(w http.ResponseWriter, r *http.Request) error {
	loggedIn, _ := alreadyLoggedIn(r)
	if !loggedIn {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}

	cookie, _ := r.Cookie("chatterfly-cookie")
	sId := cookie.Value

	err := deleteInRedis(sId)
	if err != nil {
		return internalServerError(w, err)
	}

	cookie = &http.Cookie{
		Name:   "chatterfly-cookie",
		Value:  "",
		MaxAge: -1,
	}

	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)

	return nil
}

func HandleCreateChatroom(w http.ResponseWriter, r *http.Request) error {
	loggedIn, _ := alreadyLoggedIn(r)
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil
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
			return internalServerError(w, err)
		}

		cookie, _ := r.Cookie("chatterfly-cookie")
		sId := cookie.Value

		session, err := getSession(sId)
		if err != nil {
			return internalServerError(w, err)
		}

		un := session.Username
		user, err := getUser(un)
		if err != nil {
			return internalServerError(w, err) // here I am returing internal server error because here user is not providing any username info so here we dont the cause of the error.
		}
		crs := append(user.Chatrooms, cr)

		updateCRListForUser(user.Username, crs)
		http.Redirect(w, r, fmt.Sprintf("/chatroom/c/%v/", crKey), http.StatusSeeOther)
		return nil
	}

	http.StripPrefix("/chatroom", http.FileServer(http.Dir("./public/createChatroom"))).ServeHTTP(w, r)
	return nil
}

func HandleChatroom(w http.ResponseWriter, r *http.Request) error {
	loggedIn, _ := alreadyLoggedIn(r)
	if !loggedIn {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil
	}

	cookie, _ := r.Cookie("chatterfly-cookie")
	sId := cookie.Value

	session, err := getSession(sId)
	if err != nil {
		return internalServerError(w, err)
	}

	params := mux.Vars(r)
	crKey := params["crKey"]

	cr, err := getChatRoom(crKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return fmt.Errorf("chatroom does not exists")
		} else {
			return internalServerError(w, err)
		}
	}

	if r.Method == http.MethodPost {
		un := r.PostFormValue("invite-user")
		user, err := getUser(un)
		if err != nil {
			return fmt.Errorf("user with username %s does not exists", un)
		}

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
	}

	session.ChatRoomKey = crKey
	// update the session with the chatroomkey
	err = storeInRedis(sId, session)
	if err != nil {
		return internalServerError(w, err)
	}

	http.StripPrefix("/chatroom/c/"+crKey, http.FileServer(http.Dir("./public/chatroom"))).ServeHTTP(w, r)
	return nil
}

func PathWithoutFS(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	http.Redirect(w, r, path+"/", http.StatusSeeOther)
}

func HandleConnections(w http.ResponseWriter, r *http.Request) error {
	loggedIn, _ := alreadyLoggedIn(r)
	if !loggedIn {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}

	cookie, _ := r.Cookie("chatterfly-cookie")
	sId := cookie.Value

	session, err := getSession(sId)
	if err != nil {
		return internalServerError(w, err)
	}

	un := session.Username
	fn := session.Firstname
	crKey := session.ChatRoomKey

	chat := model.Chat{}
	filter := bson.M{"key": crKey}
	cr, _ := getChatRoom(crKey)

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return internalServerError(w, err)
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
			return internalServerError(w, err)
		}
	}

	messageClient(ws, cr) // this is sending chatroom for displaying the chatroomname on top of chatroom

	user, err := getUser(un)
	if err != nil {
		return internalServerError(w, err)
	}

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

	return nil
}

func SendUserData(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	cookie, _ := r.Cookie("chatterfly-cookie")
	sId := cookie.Value

	session, err := getSession(sId)
	if err != nil {
		return internalServerError(w, err)
	}

	un := session.Username
	user, err := getUser(un)
	if err != nil {
		return internalServerError(w, err)
	}

	err = json.NewEncoder(w).Encode(user)
	if err != nil {
		return internalServerError(w, err)
	}

	return nil
}

func alreadyLoggedIn(r *http.Request) (bool, error) {
	cookie, err := r.Cookie("chatterfly-cookie")
	if err == http.ErrNoCookie {
		return false, nil
	}

	sId := cookie.Value

	_, err = rdb.Get(sId).Result()
	if err != nil {
		return false, err
	}

	return true, nil
}

func storeInRedis(key string, value interface{}) error {
	json, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if err = rdb.Set(key, json, 0).Err(); err != nil {
		return err
	}

	return nil
}

func deleteInRedis(key string) error {
	_, err := rdb.Del(key).Result()
	if err != nil {
		return err
	}

	return nil
}

func getSession(key string) (model.Session, error) {
	session := model.Session{}
	jsonObj, err := rdb.Get(key).Result()
	if err != nil {
		return session, err
	}

	err = json.Unmarshal([]byte(jsonObj), &session)
	if err != nil {
		return session, err
	}

	return session, nil
}

func storeInMongo(collection *mongo.Collection, value interface{}) error {
	_, err := collection.InsertOne(ctx, value)
	if err != nil {
		return err
	}

	return nil
}

func getUser(un string) (model.User, error) {
	user := model.User{}
	filter := bson.M{"username": un}
	err := userCollection.FindOne(context.Background(), filter).Decode(&user)
	if err != nil {
		return user, err
	}

	return user, nil
}

func getChatRoom(key string) (model.ChatRoom, error) {
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
		err := storeInMongo(chatCollection, msg)
		if err != nil {
			log.Println(err)
		}
		err = messageClients(msg)
		if err != nil {
			log.Println(err)
		}
	}
}

func MakeHTTPHandlerFunc(fn apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		}
	}
}

func createUserFromForm(w http.ResponseWriter, r *http.Request) (model.User, error) {
	user := model.User{}
	bs, err := bcrypt.GenerateFromPassword([]byte(r.PostFormValue("password")), bcrypt.MinCost)
	if err != nil {
		return user, internalServerError(w, err)
	}

	user = model.User{
		ID:        primitive.NewObjectID(),
		CreatedAt: time.Now(),
		Username:  r.PostFormValue("username"),
		Password:  bs,
		Firstname: r.PostFormValue("firstname"),
		Lastname:  r.PostFormValue("lastname"),
	}

	return user, nil
}

func createSessionFromUser(user model.User) model.Session {
	session := model.Session{
		Username:  user.Username,
		Firstname: user.Firstname,
		Lastname:  user.Lastname,
	}

	return session
}

func createCookie(w http.ResponseWriter, sID string) {
	http.SetCookie(w, &http.Cookie{
		Name:  "chatterfly-cookie",
		Value: sID,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)

	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		return internalServerError(w, err)
	}
	return nil
}

func permissionDenied(w http.ResponseWriter, errStr string) error {
	return writeJSON(w, http.StatusForbidden, apiError{Error: errStr})
}

func internalServerError(w http.ResponseWriter, err error) error {
	log.Println(err)
	return writeJSON(w, http.StatusInternalServerError, apiError{Error: "internal server error"})
}

func unsafeError(err error) bool {
	return !websocket.IsCloseError(err, websocket.CloseGoingAway) && err != io.EOF
}
