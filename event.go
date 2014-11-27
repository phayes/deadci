package main

import (
	"github.com/phayes/hookserve/hookserve"
	"io"
	"os"
	"os/exec"
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
	cmd := exec.Command(RunCommand.First(), RunCommand.Tail()...)
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

func (e *Event) Report(status string, err error) error {
	if err != nil {
		e.Log = append(e.Log, []byte("\n"+status+": "+err.Error())...)
	} else {
		e.Log = append(e.Log, []byte("\n"+status)...)
	}

	e.Status = status
	return e.Update()

	// @@TODO: Log back to github
}
