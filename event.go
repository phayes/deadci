package main

import (
	"code.google.com/p/goauth2/oauth"
	"encoding/json"
	"github.com/google/go-github/github"
	"github.com/phayes/hookserve/hookserve"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"
)

var (
	StatusPending    = "pending"
	StatusRunning    = "running"
	StatusSuccess    = "success"
	StatusFailed     = "failed"
	StatusFailedBoot = "failed-boot"
)

type Event struct {
	hookserve.Event
	ID     int
	Time   time.Time
	Domain string
	Status string
	Log    []byte
}

func (e *Event) Path() string {
	return e.Domain + "/" + e.Owner + "/" + e.Repo + "/" + e.Branch + "/" + e.Commit
}

func (e *Event) String() string {
	out := "time:   " + e.Time.String() + "\n"
	out += "domain: " + e.Domain + "\n"
	out += e.Event.String()
	out += "status: " + e.Status + "\n\n"
	out += string(e.Log)
	return out
}

// Run a test.
// This should be done inside a goroutine
func (e *Event) Run() (string, error) {
	if e.Status != StatusRunning {
		panic("Event should have it status set to `running` before calling Run()")
	}

	// Clean the scratch space
	err := os.RemoveAll(os.TempDir() + "deadci/" + e.Path())
	if err != nil {
		return StatusFailedBoot, err
	}

	// Create scratch space
	err = os.MkdirAll(os.TempDir()+"deadci/"+e.Path(), 0777)
	if err != nil {
		return StatusFailedBoot, err
	}

	// Clone repo
	cmdClone := exec.Command("git", "clone", "git@"+e.Domain+":"+e.Owner+"/"+e.Repo+".git")
	cmdClone.Dir = os.TempDir() + "deadci/" + e.Path()
	cmdCloneOut, err := cmdClone.CombinedOutput()
	e.Log = append(e.Log, cmdCloneOut...)
	if err != nil {
		return StatusFailedBoot, err
	}

	// Check out correct commit
	cmdCheckout := exec.Command("git", "checkout", "-q", e.Commit)
	cmdCheckout.Dir = os.TempDir() + "deadci/" + e.Path() + "/" + e.Repo
	cmdCheckoutOut, err := cmdCheckout.CombinedOutput()
	e.Log = append(e.Log, cmdCheckoutOut...)
	if err != nil {
		return StatusFailedBoot, err
	}

	// Run the main command to do the testing
	var cmd *exec.Cmd
	if len(Config.Command) == 1 {
		cmd = exec.Command(Config.Command[0])
	} else {
		cmd = exec.Command(Config.Command[0], Config.Command[1:]...)
	}
	cmd.Dir = os.TempDir() + "deadci/" + e.Path() + "/" + e.Repo
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "DEADCI_DOMAIN="+e.Domain, "DEADCI_OWNER="+e.Owner, "DEADCI_REPO="+e.Repo, "DEADCI_BRANCH="+e.Branch, "DEADCI_COMMIT="+e.Commit)
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return StatusFailedBoot, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return StatusFailedBoot, err
	}
	err = cmd.Start()
	if err != nil {
		return StatusFailed, err
	}
	// Collect stdout and stderr
	stdoutbuff := make([]byte, 1024)
	stderrbuff := make([]byte, 1024)

	cmd.Run()
	for {
		done := false
		//@@TODO: Move to buffio.Scanner and two seperate co-running go-routines
		//@@TODO: This could very likely be highly refactored

		// stdout
		n, err := stdoutPipe.Read(stdoutbuff)
		if err != nil {
			if err == io.EOF {
				done = true
			} else {
				return StatusFailed, err
			}
		}
		e.Log = append(e.Log, stdoutbuff[:n]...)

		// stderr
		n, err = stderrPipe.Read(stderrbuff)
		if err != nil {
			if err == io.EOF {
				done = true
			} else {
				return StatusFailed, err
			}
		}
		e.Log = append(e.Log, stderrbuff[:n]...)
		e.Update()
		if done {
			break
		}
	}

	err = cmd.Wait()
	if err != nil {
		return StatusFailed, err
	} else {
		return StatusSuccess, nil
	}
}

func (e *Event) Finalize(status string, err error) error {
	if err != nil {
		e.Log = append(e.Log, []byte("\n"+status+": "+err.Error())...)
	} else {
		e.Log = append(e.Log, []byte("\n"+status)...)
	}

	e.Status = status
	return e.Update()

	// Send the report to the provider
	err = e.Report()
	if err != nil {
		return err
	}

	return nil
}

func (e *Event) FullURL() string {
	return "http://" + Config.Host + ":" + strconv.Itoa(Config.Port) + "/" + e.Path()
}

func (e *Event) Report() error {
	if e.Domain == "github.com" {
		err := e.ReportGitHub()
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Event) ReportGitHub() error {
	// If github-token is not set, skip posting results
	if Config.Github.Token == "" {
		return nil
	}

	// Create the authorization transport
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: Config.Github.Token},
	}

	status := e.TranslateStatus()
	desc := e.StatusDescription()
	url := e.FullURL()
	repoStatus := &github.RepoStatus{
		State:       &status,
		TargetURL:   &url,
		Description: &desc,
	}

	client := github.NewClient(t.Client())
	_, _, err := client.Repositories.CreateStatus(e.Owner, e.Repo, e.Commit, repoStatus)
	if err != nil {
		return err
	}

	// Leave a comment on the commit if it failed.  We don't leave a comment if there was an error or a pass.
	if status == StatusFailed {
		commentStr := "DeadCI - build " + e.Status + ": " + desc + "\n" + "For details please see: " + e.FullURL()
		comment := &github.RepositoryComment{Body: &commentStr}
		_, _, err := client.Repositories.CreateComment(e.Owner, e.Repo, e.Commit, comment)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *Event) StatusDescription() string {
	lookup := map[string]string{
		StatusPending:    "Build queued - please wait",
		StatusRunning:    "Build and tests running - please wait",
		StatusSuccess:    "Build successful and tests passed",
		StatusFailed:     "Build testing failed",
		StatusFailedBoot: "Error bootstrapping build environment",
	}
	desc, ok := lookup[e.Status]
	if !ok {
		panic("Unknown status: " + e.Status)
	}
	return desc
}

func (e *Event) TranslateStatus() string {
	lookup := map[string]map[string]string{
		"github.com": map[string]string{
			StatusPending:    "pending",
			StatusRunning:    "pending",
			StatusSuccess:    "success",
			StatusFailed:     "failure",
			StatusFailedBoot: "error",
		},
	}
	translated, ok := lookup[e.Domain][e.Status]
	if !ok {
		panic("Unknown status: " + e.Domain + " " + e.Status)
	}
	return translated
}

func (e *Event) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"time":   e.Time.String(),
		"domain": e.Domain,
		"owner":  e.Owner,
		"repo":   e.Repo,
		"branch": e.Branch,
		"commit": e.Commit,
		"status": e.Status,
		"log":    string(e.Log),
	})
}
