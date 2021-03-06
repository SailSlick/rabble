#!/usr/bin/env sh

set -e

if [ -z "$LOCAL_USER_ID" ]
then
  echo "ERROR: environment variable LOCAL_USER_ID does not exist."
  echo "It is needed to preserve the mounted filesystem."
  echo "See build.sh in the root of the project."
  exit 1
fi

cd /repo
mkdir -p build_out/
mkdir -p build_out/activities/

# If when the script exits (because of an error or otherwise) the following
# function runs. The exit code is preserved.
function chown_trap {
  echo "Fixing permissions"
  adduser -D -u $LOCAL_USER_ID user
  cd /repo
  chown -R user build_out
  chown -R user chump/node_modules
}
trap chown_trap EXIT
trap chown_trap SIGINT

echo "Running build"

echo "Building docker-compose configs"
cp -R containers/ build_out/
# Set default value for env vars
export RABBLE_SEARCH_TYPE="${RABBLE_SEARCH_TYPE:-simple-search}"
# Use no-op container if no method of recommendation is supplied
export RABBLE_FOLLOW_RECOMMENDER_METHOD="${RABBLE_FOLLOW_RECOMMENDER_METHOD:-none}"
RABBLE_FOLLOWS_SERVICE_LOCATION="./services/noop"
if [ "$RABBLE_FOLLOW_RECOMMENDER_METHOD" != "none" ]; then
  RABBLE_FOLLOWS_SERVICE_LOCATION="./services/recommend_follows"
fi
export RABBLE_FOLLOWS_SERVICE_LOCATION

export RABBLE_POSTS_RECOMMENDER_METHOD="${RABBLE_POSTS_RECOMMENDER_METHOD:-none}"
RABBLE_POSTS_SERVICE_LOCATION="./services/noop"
if [ "$RABBLE_POSTS_RECOMMENDER_METHOD" != "none" ]; then
  RABBLE_POSTS_SERVICE_LOCATION="./services/recommend_posts"
fi
export RABBLE_POSTS_SERVICE_LOCATION

. build_out/containers/gen_first_config.sh
. build_out/containers/gen_second_config.sh

# Add build commands here.
# The current working directory is the root of the repo.
# Write your output to the `build_out` directory
cp -R services/activities/create build_out/activities/
cp -R services/activities/delete build_out/activities/
cp -R services/activities/undo build_out/activities/
cp -R services/activities/update build_out/activities/
cp -R services/activities/follow build_out/activities/
cp -R services/activities/like build_out/activities/
cp -R services/activities/announce build_out/activities/

echo "Building python protos"
python3 -m grpc_tools.protoc \
  -I. \
  --python_out=build_out/ \
  --grpc_python_out=build_out/ \
  services/proto/*.proto

echo "Building database service"
cp -R services/database build_out/

echo "Building follows service"
cp -R services/follows build_out/

echo "Building article service"
cp -R services/article build_out/

echo "Building approver service"
cp -R services/activities/approver build_out/activities/

echo "Building users service"
cp -R services/users build_out/

echo "Building recommend_follows service"
cp -R services/recommend_follows build_out/

echo "Building recommend_posts service"
cp -R services/recommend_posts build_out/

echo "Building ldnorm service"
cp -R services/ldnormaliser build_out/

echo "Building actors service"
cp -R services/actors build_out/

echo "Building logger service and lib"
cp -R services/logger build_out/
cp -R services/utils build_out/

echo "Building protos for Go"
# This generate compiled protos and place them in the proto dir.
protoc -I. --go_out=plugins=grpc:"." services/proto/*.proto

echo "Creating go workspace"
mkdir -p /go/src/github.com/cpssd/
cp -R /repo /go/src/github.com/cpssd/rabble

echo "Building all go binaries"
rm /go/bin/*
go install github.com/cpssd/rabble/...
mv /go/bin/* build_out

echo "Cleaning up go protos"
rm services/proto/*.pb.go

echo "Installing node.js dependencies"
cd chump && npm install && cd ..

echo "Building client"
if [ -z "$TEST_ENV" ]; then
  echo "TEST_ENV not set, building chump prod"
  cd chump && npm run build:prod && cd ..
else
  cd chump && npm run build && cd ..
fi

mv chump/dist build_out/chump_dist
