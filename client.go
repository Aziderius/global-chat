package main

import (
	"bytes"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	//Tiempo permitido para escribir un mensaje
	writeWait = 10 * time.Second

	//tiempo permitido para leer el siguiente mensaje pong
	pongWait = 60 * time.Second

	//manda pings al peer en este periodo de tiempo, por lo tanto, pingPeriod debe ser menor a pongWait
	pingPeriod = (pongWait * 9) / 10

	//tamaño maximo del mensaje permitido
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client es un intermediario entre el websocket y la connexion con el hub
type Client struct {
	hub *Hub

	//Conexion con el websocket
	conn *websocket.Conn

	//Buffer para escribir mensajes
	send chan []byte
}

/*
readPump escribe los mensajes desde la conexión websocket hacia el hub

La aplicacion corre readPump en una goroutine per-conexion. La aplicacion se asegura
de que al menos haya un lector en la conexion ejecutando todas las lecturas desde esta goroutine
*/
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		c.hub.broadcast <- message
	}
}

/*
wirtePump escribe los mensajes desde el hub hacia la conexion websocket

una Goroutine corre writePump por cada conexion. La aplicacion se asegura
de que haya al menos un escritor en la conexion ejecutando todas las escrituras desde esta goroutine
*/
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				//El hub cierra el canal
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			//Se añade el mensaje actual de la cola al actual mensaje del websocket
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs se encarga de los request del websocket
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	//permite la recopilacion de memoria a la que hace referencia el cliente
	//que llaama haciendo todo el trabajo en las nuevas Goroutines
	go client.writePump()
	go client.readPump()
}
