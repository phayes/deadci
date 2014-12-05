<p align="center"><img src="/media/deadci-logo.png" /></p>

DeadCI is a lightweight continuous integration and testing webserver that integrates seamlessly with GitHub (other platforms coming). As the name implies it is dead easy to use. DeadCI works by running a command of your choice at the root of the repository being built. It's easy to run [TravisCI](https://travis-ci.org) jobs from a `.travis.yml` locally. It also integrates nicely with [JoliCI](https://github.com/jolicode/JoliCi) to run yours tests inside a [Docker](https://www.docker.com) container. 

Installation is very straight forward.
```bash
$ wget "https://phayes.github.io/bin/current/deadci/linux/deadci.tar.gz"
$ tar -xf deadci.tar.gz           # Extract DeadCI from tarball
$ sudo cp deadci /usr/bin         # Copy the DeadCI executable to somewhere in your $PATH
$ sudo mkdir /etc/deadci          # Create a data-directory for DeadCI
$ cp deadci.ini /etc/deadci       # Copy default settings over to data directory
$ vim /etc/deadci/deadci.ini      # Edit .ini settings as needed
$ deadci --data-dir=/etc/deadci   # Start the DeadCI webserver
Webserver started at http://example.com
Github payload URL: http://example.com/postreceive
```

Building from source
```bash
$ sudo apt-get install golang                    # Download go. Alternativly build from source: https://golang.org/doc/install/source
$ mkdir ~/.gopath && export GOPATH=~/.gopath     # Replace with desired GOPATH
$ export PATH=$PATH:$GOPATH/bin                  # For convenience, add go's bin dir to your PATH
$ go get github.com/phayes/deadci                # Download source and compile
```

### Settings up GitHub

Setting github to work with DeadCI is easy. 

##### Step 1

Step 1 is to set-up your github webhook. Navigate to `github.com/<name>/<repo>/settings/hooks` and create a new webhook. Setting up your webhook should look something like this:

![Configuring webhooks in github](https://i.imgur.com/u3ciUD7.png)

Simply fill in the URL that DeadCI gives you when it boots, optionally setting your secret for HMAC verification.

##### Step 2

Step 2 is to set-up your github access token so DeadCI can post it's results back to github. This step is optional if you don't want to show status reports on Pull Requests and commits in github. 
