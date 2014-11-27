package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/phayes/hookserve/hookserve"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	RunCommand cli.Args
	DataDir    string
)

func main() {

	app := cli.NewApp()
	app.Name = "deadci"
	app.Usage = "Dead easy CI server\n\n"
	app.Usage += "EXAMPLES:\n"
	app.Usage += "   # Process a .travis.yml file using the travis command line tool\n"
	app.Usage += "   deadci --data-dir=/opt/deadci --port=8080 --secret=xyz travis run\n\n"
	app.Usage += "   # Run on port 80 with no HMAC verification, run a file in the repo called `runtest`\n"
	app.Usage += "   deadci --data-dir=/opt/deadci ./runtests"
	app.Version = "1.0"
	app.Author = "Patrick Hayes"
	app.Email = "patrick.d.hayes@gmail.com"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "data-dir",
			Value: "",
			Usage: "Data directory. Must be writable. ",
		},
		cli.IntFlag{
			Name:  "port, p",
			Value: 80,
			Usage: "Optional. Port on which to listen for github webhooks and serve reports, default to 80.",
		},
		cli.StringFlag{
			Name:  "secret, s",
			Value: "",
			Usage: "Optional. Secret for HMAC verification. If not provided no HMAC verification will be done and all valid requests will be processed",
		},
	}

	app.Action = func(c *cli.Context) {
		if c.Args().First() == "" {
			log.Fatal("Command argument required. Please run `deadci --help` for more information")
		}
		if c.String("data-dir") == "" {
			log.Fatal("--data-dir flag required. Please run `deadci --help` for more information")
		}

		// Set the global RunCommand
		RunCommand = c.Args()
		DataDir = c.String("data-dir")

		// Set up database and ansi2html script
		InitDB()
		InitANSI2HTML()

		// Set up HTTP paths
		githubreceive := hookserve.NewServer()
		githubreceive.Secret = c.String("secret")
		http.Handle("/postreceive", githubreceive)
		http.HandleFunc("/rerun/", handleReRun)
		http.HandleFunc("/", handleUI)

		// Listen ans serve HTTP
		go func() {
			log.Println("Listening on port " + strconv.Itoa(c.Int("port")))
			err := http.ListenAndServe(":"+strconv.Itoa(c.Int("port")), nil)
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
					}
				} else {
					// It's a new event, insert it anew
					err = event.Insert()
					if err != nil {
						log.Println(err)
					}
				}
			default:
				event, err := PopEvent()
				if err != nil {
					log.Println(err)
				} else if event != nil {
					go func() {
						status, err := event.Run()
						err = event.Report(status, err)
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

	app.Run(os.Args)
}

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

	io.Copy(w, ansi2html)

	if event.Status == StatusSuccess || event.Status == StatusFailed || event.Status == StatusFailedBoot {
		fmt.Fprintln(w, "<a href='/rerun/"+event.Path()+"'>re-run</a>")
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
	event, err := GetEvent(parts[2], parts[3], parts[4], parts[5], parts[6])
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
	event.Status = StatusRunning
	event.Log = []byte("Retrying...\n")
	event.Update()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go func() {
		status, err := event.Run()
		err = event.Report(status, err)
		if err != nil {
			log.Println(err)
		}
	}()

	http.Redirect(w, r, "/"+event.Path(), 303)
}
