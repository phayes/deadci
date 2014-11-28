package main

import (
	"errors"
	"fmt"
	"github.com/phayes/hookserve/hookserve"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func main() {

	InitConfig()
	InitDB()
	InitANSI2HTML()

	// Set up HTTP paths
	githubreceive := hookserve.NewServer()
	if Config.Github.Enabled {
		githubreceive.Secret = Config.Github.Secret
		http.Handle("/postreceive", githubreceive)
		fmt.Println("Github webhook URL: http://" + Config.Host + ":" + strconv.Itoa(Config.Port) + "/postreceive")
	}
	http.HandleFunc("/", handleUI)

	// Listen and serve HTTP
	go func() {
		fmt.Println("Listening on port " + strconv.Itoa(Config.Port))
		err := http.ListenAndServe(":"+strconv.Itoa(Config.Port), nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}()

	// Receive events from github and process them
	for {
		select {
		case commit := <-githubreceive.Events:
			event := Event{
				Event:  commit,
				Domain: "github.com", // For now we only support github
				Status: StatusPending,
				Time:   time.Now(),
			}

			// First check to see if the event already exists, and if it is reque it if it's not running
			checkEvent, err := GetEvent(event.Domain, event.Owner, event.Repo, event.Branch, event.Commit)
			if err != nil {
				log.Println(err)
			}
			if checkEvent != nil {
				// It's an old event, requeue it if we can
				if checkEvent.Status != StatusRunning {
					checkEvent.Status = StatusPending
					err = checkEvent.Update()
					if err != nil {
						log.Println(err)
					}
					err = checkEvent.Report()
					if err != nil {
						log.Println(err)
					}
				}
			} else {
				// It's a new event, insert it anew
				err = event.Insert()
				if err != nil {
					log.Println(err)
				}
				err = event.Report()
				if err != nil {
					log.Println(err)
				}
			}
		default:
			event, err := PopEvent()
			if err != nil {
				log.Println(err)
			} else if event != nil {
				err = event.Report()
				if err != nil {
					log.Println(err)
				}
				go func() {
					status, err := event.Run()
					err = event.Finalize(status, err)
					if err != nil {
						log.Println(err)
					}
				}()
			} else {
				time.Sleep(50 * time.Millisecond)
			}
		}
	}
}

// Handle regular UI requests
func handleUI(w http.ResponseWriter, r *http.Request) {

	// If it's a POST we re-run it
	if r.Method == "POST" {
		handleReRun(w, r)
		return
	}

	// Check the method
	if r.Method != "GET" {
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	parts, err := parsePath(r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	event, err := GetEvent(parts[1], parts[2], parts[3], parts[4], parts[5])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if event == nil {
		http.NotFound(w, r)
		return
	}

	ansi2html, err := ANSI2HTML(event.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	// Print output
	fmt.Fprintln(w, "<html><body class='f9 b9'>")
	io.Copy(w, ansi2html)
	if event.Status == StatusSuccess || event.Status == StatusFailed || event.Status == StatusFailedBoot {
		fmt.Fprintln(w, "<form method='POST'><input type='submit' value='re-run'></form>")
	}
	fmt.Fprintln(w, "</body></html>")
}

func handleReRun(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	parts, err := parsePath(r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	event, err := GetEvent(parts[1], parts[2], parts[3], parts[4], parts[5])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if event == nil {
		// For now we only support github
		if parts[1] != "github.com" {
			http.Error(w, "Only github.com currently supported", http.StatusInternalServerError)
		}

		// Event doesn't exist, create it and mark pending to queue it
		event := Event{
			Event: hookserve.Event{
				Owner:  parts[2],
				Repo:   parts[3],
				Branch: parts[4],
				Commit: parts[5],
			},
			Domain: "github.com", // For now we only support github
			Status: StatusPending,
			Time:   time.Now(),
		}
		err := event.Insert()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = event.Report()
		if err != nil {
			log.Println(err)
		}
		http.Redirect(w, r, "/"+event.Path(), http.StatusSeeOther)
	} else {
		if event.Status == StatusRunning {
			http.Error(w, "Unable to re-run already running item", http.StatusInternalServerError)
			return
		}

		// We have the event, run it again
		// Save it back to the database marked as running
		event.Status = StatusRunning
		event.Log = []byte("Retrying...\n")
		err := event.Update()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = event.Report()
		if err != nil {
			log.Println(err)
		}

		go func() {
			status, err := event.Run()
			err = event.Finalize(status, err)
			if err != nil {
				log.Println(err)
			}
		}()

		http.Redirect(w, r, "/"+event.Path(), 303)
	}
}

func parsePath(path string) ([]string, error) {
	parts := strings.Split(path, "/")
	if len(parts) != 6 {
		return nil, errors.New("Invalid Path")
	}
	// Filter illigal characters
	for _, part := range parts {
		if strings.ContainsAny(part, " \"\\\b\f\n\r\t\v") {
			return nil, errors.New("Illigal character in path")
		}
	}
	return parts, nil
}
