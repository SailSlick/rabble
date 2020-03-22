#!/usr/bin/env sh
set -e

# build.sh builds all of docker's build using the build_container.
# if _TEST_RABBLE is set, then it also runs the test script.

docker_tag_exists() {
  curl --silent -f -lSL https://index.docker.io/v1/repositories/$1/tags/$2 > /dev/null
}

# 1=ONLY_IMAGE, 2=IMAGE, 3=DOCKERFILE_DIR,
docker_pull_or_build() {
  DOCKERFILE_HASH="$(shasum $3/Dockerfile | head -c 40)"
  IMAGE_NAME="$2:$DOCKERFILE_HASH"
  if [ "$1" = "--only-image" ] && docker_tag_exists "$2" "$DOCKERFILE_HASH"; then
    echo "Docker tag exists, no need to recreate image"
    exit 0
  fi

  echo "Attempting to pull $IMAGE_NAME"
  if ! docker pull $IMAGE_NAME; then
    echo "Pulling image failed, building"
    docker build \
      --tag $IMAGE_NAME \
      --tag $2:latest \
      --file $3/Dockerfile \
      $3
    if [ -z "$DOCKER_PASS" ]; then
      echo "Password for docker hub not given, continuing"
    else
      docker login -u rabblenetwork -p $DOCKER_PASS
      echo "Logged in to docker hub, pushing"
      docker push $IMAGE_NAME
      docker push $1:latest
    fi
  fi
}

USER_ID=`id -u $USER`

REPO_ROOT="$(git rev-parse --show-toplevel)"
if [ -z "$REPO_ROOT" ]
then
  echo "Could not find root of repo"
  exit 1
fi

BUILD_OUT="$REPO_ROOT/build_out"
echo "RM'ing $BUILD_OUT"
rm -rf $BUILD_OUT
mkdir $BUILD_OUT


IMAGE="rabblenetwork/rabble_base"
DOCKERFILE_DIR="$REPO_ROOT/containers/base"
docker_pull_or_build "$1" "$IMAGE" "$DOCKERFILE_DIR"


IMAGE="rabblenetwork/rabble_build"
DOCKERFILE_DIR="$REPO_ROOT/containers/build_container"
docker_pull_or_build "$1" "$IMAGE" "$DOCKERFILE_DIR"

if [ "$1" = "--only-image" ]; then
  exit 0
fi

for i in "$@"
do
  case $i in
    --search-type=*)
      RABBLE_SEARCH_TYPE="${i#*=}"
      shift
      ;;
    --follow-recommender-method=*)
      RABBLE_FOLLOW_RECOMMENDER_METHOD="${i#*=}"
      shift
      ;;
    --posts-recommender-method=*)
      RABBLE_POSTS_RECOMMENDER_METHOD="${i#*=}"
      shift
      ;;
    *)
      ;;
  esac
done

echo "Running build container"
docker run \
  --rm \
  --volume $REPO_ROOT:/repo \
  -e LOCAL_USER_ID=$USER_ID \
  -e TEST_RABBLE=$_TEST_RABBLE \
  -e RABBLE_SEARCH_TYPE=$RABBLE_SEARCH_TYPE \
  -e RABBLE_FOLLOW_RECOMMENDER_METHOD="${RABBLE_FOLLOW_RECOMMENDER_METHOD}" \
  -e RABBLE_POSTS_RECOMMENDER_METHOD="${RABBLE_POSTS_RECOMMENDER_METHOD}" \
  $IMAGE

echo "Done build"
