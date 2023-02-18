package controller

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/NikhilSharmaWe/chatterfly/model"
	"github.com/go-redis/redis"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	rdb *redis.Client
)

func init() {
	rdb = model.OpenRedis()
}

func Chat(w http.ResponseWriter, r *http.Request) {
	if !alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	cookie, _ := r.Cookie("chatterfly-cookie")
	sId := cookie.Value
	un := getUsernameFromSid(sId, w)
	u, _ := getUserIfExists(w, un)
	fmt.Fprintf(w, `Hello %v`, u.Firstname)
}

func Login(w http.ResponseWriter, r *http.Request) {
	if alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
		return
	}

	if r.Method == http.MethodPost {
		user := model.User{}
		un := r.PostFormValue("username")
		pw := r.PostFormValue("password")
		user, usernameExists := getUserIfExists(w, un)
		if !usernameExists {
			log.Println("Username does not exists")
			http.Error(w, "Username does not exists", http.StatusUnauthorized)
			return
		}

		err := bcrypt.CompareHashAndPassword(user.Password, []byte(pw))
		if err != nil {
			log.Println("Username and/or password do not match")
			http.Error(w, "Username and/or password do not match", http.StatusForbidden)
			return
		}
		sId := uuid.NewV4()
		session := model.Session{
			SessionId: sId.String(),
			Username:  un,
		}
		store("session", w, session)

		http.SetCookie(w, &http.Cookie{
			Name:  "chatterfly-cookie",
			Value: sId.String(),
		})

		http.Redirect(w, r, "/chat", http.StatusSeeOther)
		return
	}
	http.ServeFile(w, r, "./public/login/index.html")
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

		sId := uuid.NewV4()
		session := model.Session{
			SessionId: sId.String(),
			Username:  un,
		}
		store("session", w, session)

		bs, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		user := model.User{
			Username:  un,
			Password:  bs,
			Firstname: fn,
			Lastname:  ln,
		}
		store("user", w, user)

		http.SetCookie(w, &http.Cookie{
			Name:  "chatterfly-cookie",
			Value: sId.String(),
		})
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
		return
	}
	http.ServeFile(w, r, "./public/signup/index.html")
}

func Logout(w http.ResponseWriter, r *http.Request) {
	if !alreadyLoggedIn(w, r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	cookie := http.Cookie{
		Name:   "chatterfly-cookie",
		Value:  "",
		MaxAge: -1,
	}

	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func alreadyLoggedIn(w http.ResponseWriter, r *http.Request) bool {

	cookie, err := r.Cookie("chatterfly-cookie")
	if err == http.ErrNoCookie {
		return false
	}
	var s model.Session
	sId := cookie.Value
	sessions, err := rdb.LRange("session", 0, -1).Result()
	if err != nil {
		http.Error(w, "Unable to get the sessions info", http.StatusInternalServerError)
	}
	for _, session := range sessions {
		err := json.Unmarshal([]byte(session), &s)
		if err != nil {
			log.Println(err)
			http.Error(w, "Unable to marshal data", http.StatusInternalServerError)
		}
		if s.SessionId == sId {
			return true
		}
	}
	return false
}

func store(obj string, w http.ResponseWriter, value interface{}) {
	json, err := json.Marshal(value)
	if err != nil {
		log.Println(err)
		http.Error(w, "Unable to marshal data", http.StatusInternalServerError)
		panic(err)
	}

	if err = rdb.RPush(obj, json).Err(); err != nil {
		log.Println(err)
		http.Error(w, "Unable to push data", http.StatusInternalServerError)
		panic(err)
	}
}

func getUserIfExists(w http.ResponseWriter, un string) (model.User, bool) {
	var u model.User
	users, err := rdb.LRange("user", 0, -1).Result()
	if err != nil {
		log.Println(err)
		http.Error(w, "Unable to get the users info", http.StatusInternalServerError)
		panic(err)
	}

	for _, user := range users {
		err := json.Unmarshal([]byte(user), &u)
		if err != nil {
			log.Println(err)
			http.Error(w, "Unable to marshal data", http.StatusInternalServerError)
			panic(err)
		}
		if u.Username == un {
			return u, true
		}
	}
	return u, false
}

func getUsernameFromSid(sId string, w http.ResponseWriter) string {
	var s model.Session
	sessions, err := rdb.LRange("session", 0, -1).Result()
	if err != nil {
		log.Println(err)
		http.Error(w, "Unable to get the sessions info", http.StatusInternalServerError)
		panic(err)
	}
	for _, session := range sessions {
		err := json.Unmarshal([]byte(session), &s)
		if err != nil {
			log.Println(err)
			http.Error(w, "Unable to marshal data", http.StatusInternalServerError)
			panic(err)
		}
		if s.SessionId == sId {
			return s.Username
		}
	}
	return ""
}
