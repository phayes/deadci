package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dlintw/goconf"
)

var Config struct {
	DataDir string
	IniFile string
	TempDir string
	Command []string
	Port    int
	Host    string
	Github  struct {
		Enabled bool
		Token   string
		Secret  string
	}
	HttpsClone bool
}

func init() {
	flag.StringVar(&Config.DataDir, "data-dir", "", "Data directory where config is stored. Must be writable.")
	flag.StringVar(&Config.IniFile, "config", "", "Direct path to deadci.ini if config is not stored in data directory.")
}

// InitConfig loads the config on startup
func InitConfig() {
	flag.Parse()

	// Parse data directory
	if Config.DataDir == "" {
		fmt.Println("  --data-dir option not specified. Please create a writable data directory and invoke deadci command with --data-dir=/path/to/data/dir")
		flag.CommandLine.PrintDefaults()
		os.Exit(2)
	}
	Config.DataDir = strings.TrimRight(Config.DataDir, "/ ")

	// Read the config file
	if Config.IniFile == "" {
		Config.IniFile = Config.DataDir + "/deadci.ini"
	}
	c, err := goconf.ReadConfigFile(Config.IniFile)
	if err != nil {
		log.Fatal(err.Error() + ". Please ensure that your deadci.ini file is readable and in place at " + Config.IniFile)
	}

	// Parse command
	cmd, err := c.GetString("", "command")
	if err != nil {
		log.Fatal(err)
	}
	cmd = strings.Trim(cmd, " ")
	if cmd == "" {
		log.Fatal("Missing command in deadci.ini. Please specify a command to run to build / test your repositories.")
	}
	Config.Command = strings.Split(cmd, " ")
	if len(Config.Command) == 0 {
		log.Fatal("Missing command in deadci.ini. Please specify a command to run to build / test your repositories.")
	}

	// Parse Port
	Config.Port, err = c.GetInt("", "port")
	if err != nil {
		log.Fatal(err)
	}

	// Parse Host
	Config.Host, err = c.GetString("", "host")
	if (err != nil && err.(goconf.GetError).Reason == goconf.OptionNotFound) || Config.Host == "" {
		Config.Host, err = os.Hostname()
		if err != nil {
			log.Fatal("Unable to determine hostname. Please specify a hostname in deadci.ini")
		}
	} else if err != nil {
		log.Fatal(err)
	}

	// Parse Temp Dir
	Config.TempDir, err = c.GetString("", "tempdir")
	if (err != nil && err.(goconf.GetError).Reason == goconf.OptionNotFound) || Config.TempDir == "" {
		Config.TempDir = os.TempDir()
	} else if err != nil {
		log.Fatal(err)
	}
	// Normalize tempdir string
	Config.TempDir = strings.TrimRight(Config.TempDir, "/")

	// Parse Github settings
	if c.HasSection("github") {
		Config.Github.Enabled, err = c.GetBool("github", "enabled")
		if err != nil && err.(goconf.GetError).Reason != goconf.OptionNotFound {
			log.Fatal(err)
		}
		if Config.Github.Enabled {
			Config.Github.Token, err = c.GetString("github", "token")
			if err != nil && err.(goconf.GetError).Reason != goconf.OptionNotFound {
				log.Fatal(err)
			}
			Config.Github.Secret, err = c.GetString("github", "secret")
			if err != nil && err.(goconf.GetError).Reason != goconf.OptionNotFound {
				log.Fatal(err)
			}
		}
	}

	// Parse clone style (git or https)
	Config.HttpsClone, err = c.GetBool("", "httpsclone")
	if err != nil && err.(goconf.GetError).Reason != goconf.OptionNotFound {
		log.Fatal(err)
	}

}
