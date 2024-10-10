package main

import (
	"flag"
	"log"
	"net/http"
)

var addr = flag.String("addr", ":8080", "http service address")

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/chat" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	/*if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}*/

	// Verificar si el username está presente en la cookie
	_, err := r.Cookie("username")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.ServeFile(w, r, "home.html")
}

// Maneja la página de login
func serveLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Obtener el username desde el formulario
		username := r.FormValue("username")
		if username == "" {
			http.Error(w, "Username is required", http.StatusBadRequest)
			return
		}

		// Crear una cookie con el username
		http.SetCookie(w, &http.Cookie{
			Name:  "username",
			Value: username,
			Path:  "/",
		})

		// Redirigir al usuario a la página del chat
		http.Redirect(w, r, "/chat", http.StatusSeeOther)
	} else {
		// Si es GET, servir la página de login
		http.ServeFile(w, r, "login.html")
	}
}

func main() {
	flag.Parse()
	hub := newHub()
	go hub.run()

	// Servir archivos estáticos
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", serveLogin)
	http.HandleFunc("/chat", serveHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
