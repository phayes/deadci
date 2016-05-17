package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/phayes/hookserve/hookserve"
)

var (
	ReRunMux   = sync.Mutex{}
	InShutdown = false
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	InitConfig()
	InitDB()
	InitANSI2HTML()
	glog.Info("Starting up.")
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

	// Launch workers for running jobs
	// We have a number of workers equal to the number of cores
	for i := 1; i <= runtime.NumCPU(); i++ {
		go func() {
			for {
				event, err := PopEvent()
				if err != nil {
					log.Println(err)
				} else if event != nil {
					err = event.Report()
					if err != nil {
						// If we can't update the status on the source-control system, then just log it and continue
						event.Log = append(event.Log, []byte(err.Error()+"\n")...)
						event.Update()
						log.Println(err)
					}
					status, err := event.Run()
					err = event.Finalize(status, err)
					if err != nil {
						log.Println(err)
					}
				}

				// Wait 100 ms then check again
				time.Sleep(100 * time.Millisecond)

				// If we are shutting down, stop processing
				if InShutdown {
					return
				}
			}
		}()
	}

	// Handle a sigint to gracefully shutdown
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, syscall.SIGINT)
	go func() {
		for _ = range sigint {
			fmt.Println("Got shutdown signal. Will shutdown when actively running jobs are finished. To shutdown immidaitely, use sigquit.")
			InShutdown = true
			// Wait until we have no running jobs then shut down.
			for {
				numEvents, err := NumEvent("running")
				if err != nil {
					log.Fatal(err)
				}
				if numEvents == 0 {
					log.Println("Shutting down")
					os.Exit(0)
				} else {
					// Wait 100 ms then check again
					time.Sleep(100 * time.Millisecond)
				}
			}
		}
	}()

	// Handle a sigquit to shut down immidiately
	sigquit := make(chan os.Signal, 1)
	signal.Notify(sigquit, syscall.SIGQUIT)
	go func() {
		for _ = range sigquit {
			fmt.Println("Got quit signal. Shutting down immidaitely. To shutdown gracefully, use sigint.")
			os.Exit(1)
		}
	}()

	// Add new events to the queue as they come in
	for commit := range githubreceive.Events {
		// Only run tets on pull-requests if there is new code to test
		if commit.Type == "pull_request" && (commit.Action != "synchronize" && commit.Action != "opened") {
			continue
		}

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
	}
}

// Handle regular UI requests
func handleUI(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	path, err := parsePath(r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// If it's a POST we re-run it
	if r.Method == "POST" {
		handleReRun(path, w, r)
		return
	}

	// Check the method
	if r.Method != "GET" {
		http.Error(w, "405 Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if len(path) == 5 { // It's a single item
		handleView(path, w, r)
	} else { // It's an index request
		handleIndex(path, w, r)
	}
}

func handleReRun(path []string, w http.ResponseWriter, r *http.Request) {
	ReRunMux.Lock()
	defer ReRunMux.Unlock()

	if len(path) != 5 {
		http.NotFound(w, r)
		return
	}

	event, err := GetEvent(path[0], path[1], path[2], path[3], path[4])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if event == nil {
		// For now we only support github
		if path[0] != "github.com" {
			http.Error(w, "Only github.com currently supported", http.StatusInternalServerError)
		}

		// Event doesn't exist, create it and mark pending to queue it
		event := Event{
			Event: hookserve.Event{
				Owner:  path[1],
				Repo:   path[2],
				Branch: path[3],
				Commit: path[4],
				Type:   "push",
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

func handleView(path []string, w http.ResponseWriter, r *http.Request) {
	event, err := GetEvent(path[0], path[1], path[2], path[3], path[4])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if event == nil {
		http.NotFound(w, r)
		return
	}

	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		jbytes, err := json.MarshalIndent(event, " ", "  ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(jbytes)
	} else { // Serve HTML
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		ansi2html, err := ANSI2HTML(event.String())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Print output
		fmt.Fprintln(w, "<html><body class='f9 b9'>")
		io.Copy(w, ansi2html)
		if event.Status == StatusSuccess || event.Status == StatusFailed || event.Status == StatusFailedBoot {
			fmt.Fprintln(w, "<form method='POST'><input type='submit' value='re-run'></form>")
		}
		fmt.Fprintln(w, "</body></html>")
	}
}

func handleIndex(path []string, w http.ResponseWriter, r *http.Request) {
	// Handle main index -- list all events
	events, err := GetEvents(path...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("Accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		jbytes, err := json.MarshalIndent(events, " ", "  ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(jbytes)
	} else {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		fmt.Fprintln(w, "<html><body style='background-color:black; color:white'><table style='width:100%'>")
		for _, event := range events {
			fmt.Fprintln(w, "<tr>")
			fmt.Fprintln(w, "<td>"+event.Time.String()+"</td><td>"+event.Status+"</td><td><a href='"+event.FullURL()+"'>"+event.Path()+"<a/></td>")
			fmt.Fprintln(w, "</tr>")
		}
		fmt.Fprintln(w, "</table></body></html>")
	}
}

func parsePath(path string) ([]string, error) {
	parts := strings.Split(path, "/")
	// If we have more than 6 parts, then we are trying to access a branch with a slash in it
	// as only part that is allowed to have a slash in it is the branch name
	if numparts := len(parts); numparts > 6 {
		rawparts := parts
		parts = make([]string, 6)
		parts[0] = rawparts[0]                               // root
		parts[1] = rawparts[1]                               // domain
		parts[2] = rawparts[2]                               // user
		parts[3] = rawparts[3]                               // repo
		parts[4] = strings.Join(rawparts[4:numparts-1], "/") // branch
		parts[5] = rawparts[numparts-1]                      // commit
	}
	// Filter illigal characters
	for _, part := range parts {
		if strings.ContainsAny(part, " \"\\\b\f\n\r\t\v") {
			return nil, errors.New("Illigal character in path")
		}
	}
	if len(parts) == 2 && parts[1] == "" {
		return make([]string, 0), nil
	} else {
		return parts[1:], nil
	}
}
