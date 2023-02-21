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
	uuid "github.com/satori/go.uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

var (
	rdb        *redis.Client
	collection *mongo.Collection
	ctx        = context.Background()
)

func init() {
	rdb = model.OpenRedis()
	collection = model.CreateMongoCollection(ctx)
}

func Signup(w http.ResponseWriter, r *http.Request) {
	if alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
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
		store(sId, w, session)

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
		createUser(w, &user)

		http.SetCookie(w, &http.Cookie{
			Name:  "chatterfly-cookie",
			Value: sId,
		})
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
		return
	}
	http.ServeFile(w, r, "./public/signup/index.html")
}

func Login(w http.ResponseWriter, r *http.Request) {
	if alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
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
		store(sId, w, session)

		http.SetCookie(w, &http.Cookie{
			Name:  "chatterfly-cookie",
			Value: sId,
		})

		http.Redirect(w, r, "/chat", http.StatusSeeOther)
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
	delete(sId, w)
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
	var s model.Session
	cookie, _ := r.Cookie("chatterfly-cookie")
	sId := cookie.Value
	get(sId, w, &s)

	var u model.User
	un := s.Username
	u = getUser(w, un)

	fmt.Fprintf(w, "Hello %v %v", u.Firstname, u.Lastname)
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
func store(key string, w http.ResponseWriter, value interface{}) {
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

func delete(key string, w http.ResponseWriter) {
	_, err := rdb.Del(key).Result()
	if err != nil {
		log.Println(err)
		http.Error(w, "Unable to delete data", http.StatusInternalServerError)
	}
}

func get(key string, w http.ResponseWriter, obj interface{}) error {
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
func createUser(w http.ResponseWriter, user *model.User) {
	_, err := collection.InsertOne(ctx, user)
	if err != nil {
		log.Println(err)
		http.Error(w, "Error while creating new user", http.StatusInternalServerError)
		panic(err)
	}
}

func getUser(w http.ResponseWriter, un string) model.User {
	user := model.User{}
	filter := bson.M{"username": un}
	err := collection.FindOne(context.Background(), filter).Decode(&user)
	if err != nil {
		return user
	}
	return user
}
