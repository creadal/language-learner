package main

import (
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"math/rand"
	"net/http"
	"regexp"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v4/pgxpool"
)

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
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		fmt.Println("config Database Fail")
		fmt.Print(err)
	}

	conn, err := pgxpool.ConnectConfig(context.Background(), config)

	return conn
}

func (s Server) MainPage(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")

	// Check if user is authenticated
	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	} else {
		username := session.Values["username"].(string)

		teacher := false
		rows, err := s.conn.Query(context.Background(), "select * from teachers where username = $1", username)
		if err != nil {
			fmt.Println(err)
		}
		for rows.Next() {
			teacher = true
		}
		rows.Close()

		var html_path string
		var users []UserData
		if !teacher {
			html_path = "./templates/main.html"
		} else {
			html_path = "./templates/teachers_main.html"
			rows, _ = s.conn.Query(context.Background(), "select users.username from users left join teachers on users.username = teachers.username where teachers.username is NULL")
			for rows.Next() {
				var _username string
				rows.Scan(&_username)
				users = append(users, UserData{Username: _username})
			}
		}
		rows.Close()

		t, err := template.ParseFiles(html_path)
		if err != nil {
			fmt.Println(err)
		}
		tasks := []TaskData{}
		rows, _ = s.conn.Query(context.Background(), "select title, body from tasks")

		var task TaskData
		added := false
		for rows.Next() {
			rows.Scan(&task.Title, &task.Body)
			rows2, _ := s.conn.Query(context.Background(), "select * from answers where username = $1 and task = $2", username, task.Title)
			done := false
			for rows2.Next() {
				done = true
			}
			task.Done = done
			tasks = append(tasks, task)
			added = true
		}
		rows.Close()

		data := MainData{
			Username: username,
			Tasks:    tasks,
			Empty:    !added,
			Users:    users,
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
	rows.Close()
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
				rows.Close()
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
		} else {
			html_path = "./templates/test_not_found.html"
		}

		t, err := template.ParseFiles(html_path)
		if err != nil {
			fmt.Println(err)
		}
		rows.Close()

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

func (s Server) Task(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	} else {
		title := r.URL.Query()["title"][0]
		username := session.Values["username"].(string)

		var body string

		row := s.conn.QueryRow(context.Background(), "select body from tasks where title = $1", title)
		row.Scan(&body)

		rows, _ := s.conn.Query(context.Background(), "select answer from answers where username = $1 and task = $2", username, title)
		var answer string
		answered := false
		for rows.Next() {
			answered = true
			rows.Scan(&answer)
		}
		rows.Close()
		data := TaskData{
			Title:  title,
			Body:   body,
			Done:   answered,
			Answer: answer,
		}
		html_path := "./templates/task.html"
		t, err := template.ParseFiles(html_path)
		if err != nil {
			fmt.Println(err)
		}

		t.Execute(w, data)
	}
}

func (s Server) TaskSubmit(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	} else {
		username := session.Values["username"].(string)
		answer := r.PostFormValue("answer")
		title := r.URL.Query()["title"][0]

		rows, _ := s.conn.Query(context.Background(), "select * from answers where username = $1 and task = $2", username, title)
		exist := false
		for rows.Next() {
			exist = true
		}
		rows.Close()
		if exist {
			s.conn.QueryRow(context.Background(), "update answers set answer = $1 where task = $2 and username = $3", answer, title, username)

			fmt.Println(answer)
		} else {
			s.conn.QueryRow(context.Background(), "insert into answers values ($1, $2, $3)", username, title, answer)
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (s Server) EditTask(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	} else {
		var title, body string
		title_l := r.URL.Query()["title"]
		if len(title_l) > 0 {
			title = title_l[0]
			rows, _ := s.conn.Query(context.Background(), "select body from tasks where title = $1", title)
			for rows.Next() {
				rows.Scan(&body)
			}
			rows.Close()
		}
		html_path := "./templates/edittask.html"
		t, err := template.ParseFiles(html_path)
		if err != nil {
			fmt.Println(err)
		}
		data := TaskData{Title: title, Body: body}

		t.Execute(w, data)
	}
}

func (s Server) SaveTask(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	} else {
		var title string
		title_l := r.URL.Query()["title"]
		if len(title_l) > 0 {
			title = title_l[0]
			rows, err := s.conn.Query(context.Background(), "delete from tasks where title = $1", title)
			if err != nil {
				fmt.Println(err)
			}
			rows.Close()
		}
		new_title := r.PostFormValue("title")
		body := r.PostFormValue("body")

		rows, err := s.conn.Query(context.Background(), "insert into tasks values ($1, $2)", new_title, body)
		if err != nil {
			fmt.Println(err)
		}
		rows.Close()
		rows, err = s.conn.Query(context.Background(), "update answers set task = $1 where task = $2", new_title, title)
		if err != nil {
			fmt.Println(err)
		}
		rows.Close()
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (s Server) UserInfo(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	} else {
		var tasks []TaskData
		username := r.URL.Query()["username"][0]

		rows, _ := s.conn.Query(context.Background(), "select title from tasks")
		for rows.Next() {
			var title string
			rows.Scan(&title)
			task := TaskData{Title: title, Done: false}
			tasks = append(tasks, task)
		}
		rows.Close()
		rows, _ = s.conn.Query(context.Background(), "select task, answer from answers where username = $1", username)
		for rows.Next() {
			var title, answer string
			rows.Scan(&title, &answer)

			for i, t := range tasks {
				if title == t.Title {
					tasks[i].Done = true
					tasks[i].Answer = answer
				}
			}
		}
		rows.Close()

		data := UserData{Username: username, Tasks: tasks}
		html_path := "./templates/userinfo.html"
		t, err := template.ParseFiles(html_path)
		if err != nil {
			fmt.Println(err)
		}

		t.Execute(w, data)
	}
}

func (s Server) DeleteTask(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "cookie-name")

	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	} else {
		title := r.URL.Query()["title"][0]
		rows, _ := s.conn.Query(context.Background(),
			"delete from tasks where title = $1", title)
		rows.Close()
		rows, _ = s.conn.Query(context.Background(),
			"delete from answers where task = $1", title)
		rows.Close()

		http.Redirect(w, r, "/", http.StatusSeeOther)
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

	s.router.HandleFunc("/task", s.Task).Methods(("GET"))
	s.router.HandleFunc("/task", s.TaskSubmit).Methods(("POST"))

	s.router.HandleFunc("/edittask", s.EditTask).Methods(("GET"))
	s.router.HandleFunc("/edittask", s.SaveTask).Methods(("POST"))
	s.router.HandleFunc("/deletetask", s.DeleteTask).Methods(("GET"))

	s.router.HandleFunc("/userinfo", s.UserInfo).Methods(("GET"))

	s.router.HandleFunc("/delete_account", s.DeleteAccount).Methods(("GET"))

	s.router.HandleFunc("/translate", s.Translate).Methods(("POST"))

	s.router.HandleFunc("/profile", s.ProfilePage).Methods(("GET"))
}
