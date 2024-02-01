package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

var db *sql.DB
var limiter = rate.NewLimiter(1, 3)

func main() {
	initDB()

	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	http.HandleFunc("/", handleListUsers)
	http.HandleFunc("/add", handleAddUser)
	http.HandleFunc("/form", handleForm)
	http.HandleFunc("/delete", handleDeleteUser)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func initDB() {
	var err error
	db, err = sql.Open("postgres", "postgres://postgres:1212@localhost/Hehe?sslmode=disable")
	if err != nil {
		log.Fatal("Ошибка при подключении к базе данных:", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("Ошибка при проверке подключения к базе данных:", err)
	}
}

func handleAddUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}
	if !limiter.Allow() {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	log := logrus.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	})

	log.Info("Добавление пользователя")

	name := r.FormValue("name")
	email := r.FormValue("email")

	var existingEmail string
	err := db.QueryRow("SELECT email FROM users WHERE email = $1", email).Scan(&existingEmail)
	if err != sql.ErrNoRows {
		if err != nil {
			log.Println("Ошибка при выполнении запроса к базе данных:", err)
			http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
			return
		}
		http.Error(w, "Пользователь с таким email уже существует", http.StatusBadRequest)
		return
	}

	_, err = db.Exec("INSERT INTO users (name, email, created_at) VALUES ($1, $2, $3)", name, email, time.Now())
	if err != nil {
		log.Println("Ошибка при добавлении пользователя в базу данных:", err)
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleListUsers(w http.ResponseWriter, r *http.Request) {
	if !limiter.Allow() {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	log := logrus.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	})

	filter := r.URL.Query().Get("filter")
	sort := r.URL.Query().Get("sort")
	page := r.URL.Query().Get("page")
	limit := 10
	offset := 0

	if p, err := strconv.Atoi(page); err == nil && p > 1 {
		offset = (p - 1) * limit
	}

	query := "SELECT * FROM users"
	if filter != "" {
		query += " WHERE email LIKE '%" + filter + "%'"
	}
	if sort != "" {
		query += " ORDER BY " + sort
	}
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	rows, err := db.Query(query)
	if err != nil {
		log.Println("Ошибка при выполнении запроса к базе данных:", err)
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User

	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt, &user.UpdatedAt, &user.DeletedAt)
		if err != nil {
			log.Println("Ошибка при сканировании результата запроса:", err)
			http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		log.Println("Ошибка при парсинге шаблона:", err)
		http.Error(w, "Внутрення ошибка сервера", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, users)
	if err != nil {
		log.Println("Ошибка при выполнении шаблона:", err)
		http.Error(w, "Внутрення ошибка сервера", http.StatusInternalServerError)
		return
	}
}

func handleForm(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("form.html")
	if err != nil {
		log.Println("Ошибка при парсинге шаблона:", err)
		http.Error(w, "Внутрення ошибка сервера", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Println("Ошибка при выполнении шаблона:", err)
		http.Error(w, "Внутрення ошибка сервера", http.StatusInternalServerError)
		return
	}
}

func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	if !limiter.Allow() {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	log := logrus.WithFields(logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	})

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Не указан ID пользователя для удаления", http.StatusBadRequest)
		return
	}

	_, err := db.Exec("DELETE FROM users WHERE id = $1", id)
	if err != nil {
		log.Println("Ошибка при удалении пользователя из базы данных:", err)
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type User struct {
	ID        int
	Name      string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}
