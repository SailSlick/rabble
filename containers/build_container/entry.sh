#!/usr/bin/env sh

set -e

export LOCAL_USER_ID
export TEST_RABBLE

/repo/containers/build_container/build.sh

if [ -n "${TEST_RABBLE}" ]
then
  /repo/containers/build_container/test.sh
fi
