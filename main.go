package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/davecgh/go-spew/spew"
	"github.com/phayes/hookserve/hookserve"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

var OptCommand string = "ls -lah"

func main() {

	spew.Config.DisableMethods = true

	DBInit()

	app := cli.NewApp()
	app.Name = "lightci"
	app.Usage = "Dead easy CI server\n\n"
	app.Version = "1.0"
	app.Author = "Patrick Hayes"
	app.Email = "patrick.d.hayes@gmail.com"
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "port, p",
			Value: 80,
			Usage: "port on which to listen for github webhooks",
		},
		cli.StringFlag{
			Name:  "secret, s",
			Value: "",
			Usage: "Secret for HMAC verification. If not provided no HMAC verification will be done and all valid requests will be processed",
		},
	}

	app.Action = func(c *cli.Context) {
		githubreceive := hookserve.NewServer()
		githubreceive.Secret = c.String("secret")

		http.Handle("/postreceive", githubreceive)
		http.HandleFunc("/rerun/", handleReRun)
		http.HandleFunc("/", handleUI)

		go func() {
			log.Println("Listening on port 12345")
			err := http.ListenAndServe(":12345", nil)
			if err != nil {
				log.Fatal("ListenAndServe: ", err)
			}
		}()

		for {
			select {
			case commit := <-githubreceive.Events:
				event := Event{
					Event:  commit,
					Domain: "github.com", // For now we only support github
					Status: StatusPending,
				}
				Mux.Lock()
				err := event.Insert()
				if err != nil {
					log.Println(err)
				}
				Mux.Unlock()
			default:
				Mux.Lock()
				event, err := PopEvent()
				Mux.Unlock()
				if err != nil {
					log.Println(err)
				} else if event != nil {
					go func() {
						status, err := event.Run()
						Mux.Lock()
						event.Log = append(event.Log, []byte("\n"+string(status)+":"+err.Error())...)
						event.Status = status
						event.Update()
						Mux.Unlock()

						// @@TODO: Log back to github
					}()
				} else {
					time.Sleep(50 * time.Millisecond)
				}
			}
		}
	}

	app.Run(os.Args)
}

func handleUI(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 6 {
		http.NotFound(w, r)
		return
	}
	// Filter illigal characters
	for _, part := range parts {
		if strings.ContainsAny(part, " \"\\\b\f\n\r\t\v") {
			http.NotFound(w, r)
			return
		}
	}

	event, err := GetEvent(parts[1], parts[2], parts[3], parts[4], parts[5])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	if event == nil {
		http.NotFound(w, r)
		return
	}

	fmt.Fprintln(w, event.String())
	if event.Status == StatusSuccess || event.Status == StatusFailed || event.Status == StatusFailedBoot {
		fmt.Fprintln(w, "<a href='/rerun/"+event.Path()+"'>Rerun</a>")
	}
}

func handleReRun(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 7 {
		http.NotFound(w, r)
		return
	}
	// Filter illigal characters
	for _, part := range parts {
		if strings.ContainsAny(part, " \"\\\b\f\n\r\t\v") {
			http.NotFound(w, r)
			return
		}
	}
	Mux.Lock()
	event, err := GetEvent(parts[2], parts[3], parts[4], parts[5], parts[6])
	Mux.Unlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	if event == nil {
		http.NotFound(w, r)
		return
	}
	if event.Status == StatusRunning {
		http.Error(w, "Unable to re-run already running item", http.StatusInternalServerError)
		return
	}

	// We have the event, run it again

	// Save it back to the database marked as running
	Mux.Lock()
	event.Status = StatusRunning
	event.Log = []byte("Retrying...\n")
	err = Col.Update(event.ID, event.DBItem())
	Mux.Unlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go func() {
		status, err := event.Run()
		Mux.Lock()
		event.Log = append(event.Log, []byte("\n"+string(status)+":"+err.Error())...)
		event.Status = status
		event.Update()
		Mux.Unlock()

		// @@TODO: Log back to github
	}()

	http.Redirect(w, r, "/"+event.Path(), 303)
}
