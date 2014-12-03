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
Github webhook URL: http://example.com/postreceive
```

Building from source
```bash
$ sudo apt-get install golang                    # Download go. Alternativly build from source: https://golang.org/doc/install/source
$ mkdir ~/.gopath && export GOPATH=~/.gopath     # Replace with desired GOPATH
$ export PATH=$PATH:$GOPATH/bin                  # For convenience, add go's bin dir to your PATH
$ go get github.com/phayes/deadci                # Download source and compile
```

