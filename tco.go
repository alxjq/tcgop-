package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB
var clients = make(map[string]net.Conn)
var mu sync.Mutex

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./users.db")
	if err != nil {
		log.Fatal(err)
	}

	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE,
		password TEXT,
		is_admin INTEGER DEFAULT 0,
		is_banned INTEGER DEFAULT 0
	);`
	_, err = db.Exec(query)
	if err != nil {
		log.Fatal(err)
	}

	// Default admin user
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("adminalex123"), bcrypt.DefaultCost)
	db.Exec(`
	INSERT OR IGNORE INTO users (username, password, is_admin)
	VALUES (?, ?, 1)
	`, "alex", string(hashedPassword))
}

func broadcast(sender, message string) {
	mu.Lock()
	defer mu.Unlock()
	for username, conn := range clients {
		if username != sender {
			conn.Write([]byte(sender + "> " + message + "\n"))
		}
	}
}

func privateMessage(sender, target, message string) {
	mu.Lock()
	defer mu.Unlock()
	if conn, ok := clients[target]; ok {
		conn.Write([]byte("(private) " + sender + "> " + message + "\n"))
	}
}

func listOnline(conn net.Conn) {
	mu.Lock()
	defer mu.Unlock()
	conn.Write([]byte("Online users:\n"))
	for username := range clients {
		conn.Write([]byte("- " + username + "\n"))
	}
}

func handleClient(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	conn.Write([]byte("Connected to server\n"))
	conn.Write([]byte("1) Register\n2) Login\n> "))

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	conn.Write([]byte("Username: "))
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	conn.Write([]byte("Password: "))
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	var storedPassword string
	var isAdmin, isBanned int

	if choice == "1" {
		hashed, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		_, err := db.Exec("INSERT INTO users(username,password) VALUES(?,?)",
			username, string(hashed))
		if err != nil {
			conn.Write([]byte("User already exists.\n"))
			return
		}
		conn.Write([]byte("Registered successfully.\n"))
	} else {
		err := db.QueryRow("SELECT password,is_admin,is_banned FROM users WHERE username=?",
			username).Scan(&storedPassword, &isAdmin, &isBanned)
		if err != nil {
			conn.Write([]byte("User not found.\n"))
			return
		}

		if bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password)) != nil {
			conn.Write([]byte("Wrong password.\n"))
			return
		}

		if isBanned == 1 {
			conn.Write([]byte("You are banned.\n"))
			return
		}

		conn.Write([]byte("Login successful.\n"))
	}

	mu.Lock()
	clients[username] = conn
	mu.Unlock()

	broadcast(username, "joined the chat")
	conn.Write([]byte("Welcome " + username + "\n"))
	fmt.Println(username + " joined the chat")

	for {
		msg, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		msg = strings.TrimSpace(msg)
		if msg == "" {
			continue
		}

		// Commands
		if strings.HasPrefix(msg, "/online") {
			listOnline(conn)
			continue
		}

		if strings.HasPrefix(msg, "/msg ") {
			parts := strings.SplitN(msg, " ", 3)
			if len(parts) == 3 {
				privateMessage(username, parts[1], parts[2])
			}
			continue
		}

		if isAdmin == 1 && strings.HasPrefix(msg, "/ban ") {
			target := strings.TrimPrefix(msg, "/ban ")
			_, err := db.Exec("UPDATE users SET is_banned=1 WHERE username=?", target)
			if err != nil {
				conn.Write([]byte("Ban failed.\n"))
			} else {
				conn.Write([]byte(target + " has been banned.\n"))
			}
			continue
		}

		if isAdmin == 1 && strings.HasPrefix(msg, "/unban ") {
			target := strings.TrimPrefix(msg, "/unban ")
			_, err := db.Exec("UPDATE users SET is_banned=0 WHERE username=?", target)
			if err != nil {
				conn.Write([]byte("Unban failed.\n"))
			} else {
				conn.Write([]byte(target + " has been unbanned.\n"))
			}
			continue
		}

		// Own message
		conn.Write([]byte("you> " + msg + "\n"))

		// Server log
		fmt.Printf("[%s] %s\n", username, msg)

		// Broadcast to others
		broadcast(username, msg)
	}

	// User disconnected
	conn.Write([]byte("you> You left the chat.\n"))
	mu.Lock()
	delete(clients, username)
	mu.Unlock()
	broadcast(username, "left the chat")
	fmt.Println(username + " disconnected")
}

func main() {
	initDB()

	listener, err := net.Listen("tcp", ":1342")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	fmt.Println("Server running on all interfaces (0.0.0.0:1342)")
	fmt.Println("Check your IP address and connect from your phone or computer.")
	fmt.Println("PID:", os.Getpid())

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleClient(conn)
	}
}
