Travis-Local
============

Travis-local is a small bash script that lets you run a `.travis.yml` file locally. It only supports a subset of .travis.yml directives - specifically it will run the build lifecycle which includes before_install, install, before_script, script, after_success, after_failure, and after_script directives. It doesn't have support for any other directives, although pull-requests to add support for more directives are very welcome.

Running travis-local is easy. 

```bash
$ travis-local /path/to/repository/to/test
$ travis-local # Run in the current directory
```

You may also be interested in [JoliCi](https://github.com/jolicode/JoliCi), which let's you run your travis builds locally inside of [Docker](https://www.docker.com).

###Installing
```bash
# Install dependancies: travis and travis-build
gem install travis #install travis
git clone git@github.com:travis-ci/travis-build.git # Clone travis build
ln -s travis-build ~/.travis/travis-build

# Copy to bin path and chmod it
sudo cp travis-local /usr/bin
sudo chmod a+x /usr/bin/travis-local
```
