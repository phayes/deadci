package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/phayes/hookserve/hookserve"
	"log"
	"net/http"
	"os"
	"time"
)

var OptCommand string = "./runtests"

func main() {
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
				err := event.Insert()
				if err != nil {
					log.Println(err)
				}
			default:
				event, err := PopEvent()
				if err != nil {
					log.Println(err)
				} else if event != nil {
					go func() {
						status, err := event.Run()
						event.Log = append(event.Log, []byte("\n"+string(status)+":"+err.Error())...)
						event.Status = status
						event.Update()

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
	fmt.Fprintf(w, "Hello there")
}
