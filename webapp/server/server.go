package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"regexp"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v4/pgxpool"
)

type ViewData struct {
	Username string
}

type TranslationData struct {
	Word        string
	Translation string
}

type TestData struct {
	Word string
}

type TestCheckData struct {
	Translation string
}

type ProfileData struct {
	Username   string
	Translated int
	Learned    int
	Unknown    int
}

// Server
type Server struct {
	store  sessions.CookieStore
	conn   *pgxpool.Pool
	router *mux.Router
}

var (
	key      = []byte("1FOXCKBJU59WKCVV")
	dict_key = "dict.1.1.20220622T083522Z.7578a2a85060a599.969abc7a3236f79df102fc8d2e529f5825daf073"
)

func newServer(db_url string) *Server {
	s := &Server{
		router: mux.NewRouter(),
		store:  *sessions.NewCookieStore(key),
		conn:   connect_db(db_url),
	}

	s.createRouter()

	return s
}

func connect_db(url string) *pgxpool.Pool {
	var err error
	conn, err := pgxpool.Connect(context.Background(), url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connection to database: %v\n", err)
		os.Exit(1)
	}
	return conn
}

func (s Server) MainPage(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")

	// Check if user is authenticated
	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	} else {
		html_path := "./templates/main.html"
		t, err := template.ParseFiles(html_path)
		if err != nil {
			fmt.Println(err)
		}
		data := ViewData{
			Username: session.Values["username"].(string),
		}

		t.Execute(w, data)
	}
}

func (s Server) LoginPage(w http.ResponseWriter, r *http.Request) {
	html_path := "./templates/login.html"
	t, err := template.ParseFiles(html_path)
	if err != nil {
		fmt.Println(err)
	}

	t.Execute(w, nil)
}

func (s Server) LoginProcedure(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")

	username := r.PostFormValue("username")
	password := r.PostFormValue("password")

	row := s.conn.QueryRow(context.Background(),
		"select password from users where username = $1", username)

	logged := false
	var true_password string
	err := row.Scan(&true_password)
	if err == nil {
		fmt.Println(true_password)
		if true_password == password {
			logged = true
			session.Values["username"] = username
			session.Values["authenticated"] = true
			session.Save(r, w)
			fmt.Println(r.PostFormValue("username"))
			http.Redirect(w, r, "/", http.StatusSeeOther)
		}
	} else {
		fmt.Println(err)
	}

	if !logged {
		html_path := "./templates/login_failed.html"
		t, err := template.ParseFiles(html_path)
		if err != nil {
			fmt.Println(err)
		}

		t.Execute(w, nil)
	}
}

func (s Server) LogoutProcedure(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")

	session.Values["authenticated"] = false
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s Server) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")

	session.Values["authenticated"] = false

	username := session.Values["username"]
	_, err := s.conn.Query(context.Background(),
		"delete from users where username = $1", username)

	if err != nil {
		fmt.Println(err)
	}

	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s Server) RegisterPage(w http.ResponseWriter, r *http.Request) {
	html_path := "./templates/register.html"
	t, err := template.ParseFiles(html_path)
	if err != nil {
		fmt.Println(err)
	}

	t.Execute(w, nil)
}

func (s Server) RegisterProcedure(w http.ResponseWriter, r *http.Request) {
	var html_path string
	username := r.PostFormValue("username")
	rows, _ := s.conn.Query(context.Background(),
		"select username from users")

	found := false
	var existing_username string
	for rows.Next() {
		err := rows.Scan(&existing_username)
		if err == nil {
			if username == existing_username {
				found = true
			}
		}
	}
	if !found {
		password := r.PostFormValue("password")
		_, err := s.conn.Query(context.Background(),
			"insert into users values ($1, $2)", username, password)

		if err != nil {
			fmt.Println(err)
		}

		html_path = "./templates/register_successful.html"
	} else {
		html_path = "./templates/register_failed_taken.html"
	}

	t, err := template.ParseFiles(html_path)
	if err != nil {
		fmt.Println(err)
	}

	t.Execute(w, nil)
}

func (s Server) SettingsPage(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "This is settings page")
}

func (s Server) ProfilePage(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	} else {
		username := session.Values["username"].(string)
		row := s.conn.QueryRow(context.Background(), "select count(word) from words where username = $1 group by username", username)
		var translated int
		row.Scan(&translated)
		row = s.conn.QueryRow(context.Background(), "select count(word) from words where username = $1 and level = 3 group by username", username)
		var learned int
		row.Scan(&learned)
		row = s.conn.QueryRow(context.Background(), "select count(word) from words where username = $1 and level < 3 group by username", username)
		var unknown int
		row.Scan(&unknown)

		data := ProfileData{Username: username, Translated: translated, Learned: learned, Unknown: unknown}

		html_path := "./templates/profile.html"
		t, err := template.ParseFiles(html_path)
		if err != nil {
			fmt.Println(err)
		}

		t.Execute(w, data)
	}
}

func (s Server) UsersPage(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "This is users page")
}

func (s Server) LessonPage(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "This is lesson page")
}

func (s Server) Translate(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	} else {
		word := r.PostFormValue("word")
		url := fmt.Sprintf("https://dictionary.yandex.net/api/v1/dicservice/lookup?key=%s&lang=de-ru&text=%s", dict_key, word)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Println(err)
		}

		username := session.Values["username"]

		re, _ := regexp.Compile(`<text>(.*?)</text>`)
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println(err)
		}

		names := re.FindAllString(string(body), 2)
		fmt.Println(names)

		var html_path string
		var tr TranslationData

		if len(names) > 0 {
			html_path = "./templates/translation.html"

			tr = TranslationData{
				Word:        names[0][6 : len(names[0])-7],
				Translation: names[1][6 : len(names[1])-7],
			}

			var db_username string
			var level int
			found := false
			rows, err := s.conn.Query(context.Background(), "select username, level from words where word = $1", word)
			if err == nil {
				for rows.Next() {
					rows.Scan(&db_username, &level)

					if username == db_username {
						found = true
						if level > 0 {
							level--
							_, err = s.conn.Query(context.Background(), "update words set level = $1 where word = $2 and username = $3", level, word, username)
						}
					}
				}
				if !found {
					_, err = s.conn.Query(context.Background(), "insert into words values ($1, $2, $3, 1)", username, word, names[1][6:len(names[1])-7])
				}
			} else {
				fmt.Println(err)
			}
		} else {
			html_path = "./templates/translation_failed.html"
		}

		t, err := template.ParseFiles(html_path)
		if err != nil {
			fmt.Println(err)
		}

		t.Execute(w, tr)
	}

}

func (s Server) Test(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")
	var translation string
	found := false

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	} else {
		var words [][]string
		username := session.Values["username"]

		rows, err := s.conn.Query(context.Background(), "select word, translation, level from words where username = $1", username)
		if err == nil {
			var word string
			var level int
			for rows.Next() {
				rows.Scan(&word, &translation, &level)
				level = 3 - level
				for i := 0; i < level; i++ {
					found = true
					words = append(words, []string{word, translation})
				}
			}
		} else {
			fmt.Println(err)
		}
		var html_path string
		var data TestData
		if found {
			html_path = "./templates/test.html"
			word := words[rand.Intn(len(words))]
			data = TestData{Word: word[0]}
			session.Values["translation"] = word[1]
			session.Save(r, w)
			fmt.Println(session.Values["translation"].(string))
		} else {
			html_path = "./templates/test_not_found.html"
		}

		t, err := template.ParseFiles(html_path)
		if err != nil {
			fmt.Println(err)
		}

		t.Execute(w, data)
	}
}

func (s Server) TestCheck(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")
	var html_path string
	var data TestCheckData

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	} else {
		translation := session.Values["translation"].(string)
		given_translation := r.PostFormValue("translation")

		row := s.conn.QueryRow(context.Background(), "select level from words where username = $1 and translation = $2",
			session.Values["username"].(string), translation)
		var level int
		row.Scan(&level)

		if translation == given_translation {
			html_path = "./templates/test_correct.html"
			if level < 3 {
				level++
			}
		} else {
			html_path = "./templates/test_incorrect.html"
			data = TestCheckData{Translation: translation}
			if level > 0 {
				level--
			}
		}

		s.conn.Query(context.Background(), "update words set level = $1 where username = $2 and translation = $3",
			level, session.Values["username"].(string), translation)

		t, err := template.ParseFiles(html_path)
		if err != nil {
			fmt.Println(err)
		}

		t.Execute(w, data)
	}
}

func (s Server) createRouter() {
	s.router.HandleFunc("/", s.MainPage).Methods(("GET"))

	s.router.HandleFunc("/login", s.LoginPage).Methods(("GET"))
	s.router.HandleFunc("/login", s.LoginProcedure).Methods(("POST"))
	s.router.HandleFunc("/logout", s.LogoutProcedure).Methods(("GET"))

	s.router.HandleFunc("/register", s.RegisterPage).Methods(("GET"))
	s.router.HandleFunc("/register", s.RegisterProcedure).Methods(("POST"))

	s.router.HandleFunc("/test", s.Test).Methods(("GET"))
	s.router.HandleFunc("/test", s.TestCheck).Methods(("POST"))

	s.router.HandleFunc("/delete_account", s.DeleteAccount).Methods(("GET"))

	s.router.HandleFunc("/translate", s.Translate).Methods(("POST"))

	s.router.HandleFunc("/settings", s.SettingsPage).Methods(("GET"))
	s.router.HandleFunc("/profile", s.ProfilePage).Methods(("GET"))
	s.router.HandleFunc("/users", s.UsersPage).Methods(("GET"))
	s.router.HandleFunc("/lesson", s.LessonPage).Methods(("GET"))
}
