#!/bin/sh
# Runs on the server. Pulls the given image tag, starts it, and rolls back to the
# previously deployed tag if the site does not become healthy.
#
#   deploy.sh <image-repo> <tag>
#   e.g. deploy.sh <instance>.cn-hangzhou.personal.cr.aliyuncs.com/<namespace>/dipwatch 3f2c1ab
#
# All images live in one repository; the tag names the component: core-<sha>, web-<sha> ...
set -eu

REPO="$1"
TAG="$2"
cd "$(dirname "$0")"

[ -f .env ] || { echo "deploy: $(pwd)/.env is missing, see .env.example" >&2; exit 1; }
PORT="$(grep '^HTTP_PORT=' .env | cut -d= -f2)"
PORT="${PORT:-18080}"
PREVIOUS="$(cat .deployed_tag 2>/dev/null || true)"

start() {
	export IMAGE_REPO="$REPO" IMAGE_TAG="$1" POSTGRES_IMAGE="$REPO:postgres-16"
	docker compose -f compose.yaml pull --quiet
	docker compose -f compose.yaml up -d --no-build --remove-orphans
}

healthy() {
	i=0
	while [ "$i" -lt 30 ]; do
		if curl -fsS "http://127.0.0.1:$PORT/api/healthz" 2>/dev/null | grep -q '"db":"ok"'; then
			return 0
		fi
		i=$((i + 1))
		sleep 2
	done
	return 1
}

echo "deploy: $TAG (previous: ${PREVIOUS:-none})"
start "$TAG"

if healthy; then
	echo "$TAG" > .deployed_tag
	docker image prune -f >/dev/null
	echo "deploy: $TAG is live on port $PORT"
	exit 0
fi

echo "deploy: $TAG failed its health check" >&2
docker compose -f compose.yaml ps >&2
if [ -n "$PREVIOUS" ] && [ "$PREVIOUS" != "$TAG" ]; then
	echo "deploy: rolling back to $PREVIOUS" >&2
	start "$PREVIOUS"
	healthy && echo "deploy: rolled back to $PREVIOUS" >&2
fi
exit 1
