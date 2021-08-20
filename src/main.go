package main

import (
	"encoding/json"
	"errors"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

type httpErr struct {
	Msg  string `json:"msg"`
	Code int    `json:"code"`
}

type wsRequest struct {
	Type string `json:"type"`
	Msg  string `json:"msg"`
}

type wsWriter struct {
	con  *websocket.Conn
	Type string
}

func (w wsWriter) Write(data []byte) (int, error) {
	err := w.con.WriteJSON(wsRequest{Type: w.Type, Msg: string(data)})
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

func handleErr(w http.ResponseWriter, err error, status int) {
	msg, err := json.Marshal(&httpErr{
		Msg:  err.Error(),
		Code: status,
	})
	if err != nil {
		msg = []byte(err.Error())
	}
	http.Error(w, string(msg), status)
}

func serveWebsocket(w http.ResponseWriter, r *http.Request) {
	con, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		handleErr(w, err, http.StatusInternalServerError)
		return
	}
	defer con.Close()
	for {
		running := false
		mt, msg, err := con.ReadMessage()
		if err != nil {
			handleErr(w, err, http.StatusInternalServerError)
			break
		}
		if mt != websocket.TextMessage {
			handleErr(w, errors.New("only text message support"), http.StatusNotImplemented)
			break
		}
		var request wsRequest
		err = json.Unmarshal(msg, &request)
		if err != nil {
			handleErr(w, err, http.StatusInternalServerError)
		}
		switch request.Type {
		case "run":
			if !running {
				running = true
				fileName := "ddpFiles/" + time.Now().String() + ".ddp"
				fileName = strings.ReplaceAll(fileName, " ", "_")
				fileName = strings.ReplaceAll(fileName, ".", "_")
				fileName = strings.ReplaceAll(fileName, ":", "_")

				file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
				if err != nil {
					log.Println(err)
					err = con.WriteJSON(wsRequest{Type: "error", Msg: "file creation failed"})
					if err != nil {
						handleErr(w, err, http.StatusInternalServerError)
						continue
					}
					continue
				}
				_, err = file.Write([]byte(request.Msg))
				if err != nil {
					err = con.WriteJSON(wsRequest{Type: "error", Msg: "file writing failed"})
					if err != nil {
						handleErr(w, err, http.StatusInternalServerError)
						continue
					}
					continue
				}

				file.Close()

				cmd := exec.Command("./ddp++.exe", fileName)
				cmd.Stdout = wsWriter{con: con, Type: "stdout"}
				cmd.Stderr = wsWriter{con: con, Type: "stderr"}

				con.WriteJSON(wsRequest{Type: "started", Msg: ""})
				done := make(chan error)
				go func() {
					cmd.Start()
					done <- cmd.Wait()
				}()
				timeout := time.NewTimer(time.Second * 5)

				select {
				case <-timeout.C:
					err = cmd.Process.Kill()
					if err != nil {
						log.Println("Failed to kill ddp program")
					}
					con.WriteJSON(wsRequest{Type: "stderr", Msg: "Program was killed due to timeout"})
					break
				case err = <-done:
					if err != nil {
						con.WriteJSON(wsRequest{Type: "stderr", Msg: "Program exited with error"})
					} else {
						con.WriteJSON(wsRequest{Type: "stdout", Msg: "Program ran successful"})
					}
					break
				}
				con.WriteJSON(wsRequest{Type: "stopped", Msg: ""})

				err = os.Remove(fileName)
				if err != nil {
					log.Println("Unable to delete file: " + err.Error())
				}
			}
		case "input":
			err = con.WriteJSON(wsRequest{Type: "stdout", Msg: "taking input"})
		case "close":
			return
		default:
			err = con.WriteJSON(wsRequest{Type: "error", Msg: "unknown websocket request"})
		}
		if err != nil {
			handleErr(w, err, http.StatusInternalServerError)
		}
	}
}

var templ *template.Template = template.Must(template.ParseFiles("index.html"))

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if err := templ.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
	}
}

func main() {
	server := makeServer()

	log.Fatal(server.ListenAndServe())
}

func makeServer() http.Server {
	router := http.NewServeMux()
	router.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	router.HandleFunc("/ws", serveWebsocket)
	router.HandleFunc("/", serveIndex)

	return http.Server{Addr: ":3000", Handler: router}
}
