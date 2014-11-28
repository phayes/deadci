#!/bin/bash

# Export constant environment variables
export CI=true
export TRAVIS=true
export CONTINUOUS_INTEGRATION=true
export RAILS_ENV=test
export RACK_ENV=test
export MERB_ENV=test

# Export the current branch into variable
export TRAVIS_BRANCH=$(git symbolic-ref -q HEAD)
export TRAVIS_BRANCH=${cur_branch##refs/heads/}
export TRAVIS_BRANCH=${cur_branch:-HEAD}

# Export the current commit hash into variable
export TRAVIS_COMMIT=$(git rev-parse HEAD)

# Before-install
travis run before_install
BEFORE_INSTALL_RESULT=$?
if [ ! $BEFORE_INSTALL_RESULT -eq 0 ]; then
  travis run after_failure
  travis run after_script
  exit $BEFORE_INSTALL_RESULT
fi

# Install
travis run install
INSTALL_RESULT=$?
if [ ! $INSTALL_RESULT -eq 0 ]; then
  travis run after_failure
  travis run after_script
  exit $INSTALL_RESULT
fi

# Run tests
# SHOULD BE: travis run -p script | source /dev/stdin
# Due to a bug in bash 3.2 the above doens't work. When we upgrade to bash 4.x we can do the above. For now we have to create a file
travis run -p script > .runscript.bash
source .runscript.bash
rm .runscript.bash

# Run after_success or after_failure
if [ $TRAVIS_TEST_RESULT -eq 0 ]; then
  travis run after_success
else
  travis run after_failure
fi

# Run the final after_script
travis run after_script

# Exit with the test result status
exit $TRAVIS_TEST_RESULT
