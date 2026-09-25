#!/usr/bin/env bash
# Assemble indexes from already-tested images; never rebuild during publication.
set -euo pipefail

digests_dir=${1:?usage: publish-images.sh DIGESTS_DIRECTORY}
: "${APP_IMAGE:?}" "${WORKER_IMAGE:?}" "${GITHUB_RUN_ID:?}" "${GITHUB_RUN_ATTEMPT:?}"

build_index() {
  local repository=$1 kind=$2 arch digest
  local sources=() digests=()
  for arch in amd64 arm64; do
    digest=$(< "$digests_dir/$arch-$kind")
    if [[ ! "$digest" =~ ^sha256:[a-f0-9]{64}$ ]]; then
      echo "Invalid $arch $kind digest" >&2
      return 1
    fi
    sources+=("$repository@$digest")
    digests+=("$digest")
  done
  local tag="$repository:ci-${GITHUB_RUN_ID}-${GITHUB_RUN_ATTEMPT}"
  docker buildx imagetools create --tag "$tag" "${sources[@]}" >&2 || return
  digest=$(docker buildx imagetools inspect "$tag" --format '{{.Manifest.Digest}}') || return
  # Verify the index contains exactly the tested digest for each native platform.
  docker buildx imagetools inspect "$repository@$digest" --raw | jq -e \
    --arg amd64 "${digests[0]}" --arg arm64 "${digests[1]}" '
      (.manifests | length) == 2 and
      any(.manifests[]; .platform.os == "linux" and .platform.architecture == "amd64" and .digest == $amd64) and
      any(.manifests[]; .platform.os == "linux" and .platform.architecture == "arm64" and .digest == $arm64)
    ' >/dev/null || return
  printf '%s\n' "$digest"
}

app_digest=$(build_index "$APP_IMAGE" app)
worker_digest=$(build_index "$WORKER_IMAGE" worker)
printf 'Verified multi-platform indexes: app=%s worker=%s\n' "$app_digest" "$worker_digest"
if [[ "${PUBLISH_IMAGES:-false}" != "true" ]]; then
  echo 'Verification only; public tags are unchanged.'
  exit 0
fi

ensure_immutable() {
  local repository=$1 digest=$2
  local tag="$repository:sha-${GITHUB_SHA:?}" existing
  existing=$(docker buildx imagetools inspect "$tag" --format '{{.Manifest.Digest}}' 2>/dev/null || true)
  if [[ -n "$existing" && "$existing" != "$digest" ]]; then
    echo "$tag already points to $existing, refusing to overwrite it with $digest" >&2
    return 1
  fi
  if [[ -z "$existing" ]]; then
    docker buildx imagetools create --prefer-index=false --tag "$tag" "$repository@$digest"
  fi
}

promote() {
  local repository=$1 digest=$2
  shift 2
  local args=(--prefer-index=false) alias
  for alias in "$@"; do args+=(--tag "$repository:$alias"); done
  docker buildx imagetools create "${args[@]}" "$repository@$digest"
}

# Both architecture indexes and immutable references precede public alias changes.
ensure_immutable "$APP_IMAGE" "$app_digest"
ensure_immutable "$WORKER_IMAGE" "$worker_digest"
if [[ "${GITHUB_REF_TYPE:?}" == "tag" ]]; then
  aliases=("${GITHUB_REF_NAME#v}")
elif [[ "${GITHUB_EVENT_NAME:?}" == "workflow_dispatch" ]]; then
  aliases=(manual)
else
  aliases=(latest edge)
fi
# Registry operations across two repositories are not atomic: worker moves first.
promote "$WORKER_IMAGE" "$worker_digest" "${aliases[@]}"
promote "$APP_IMAGE" "$app_digest" "${aliases[@]}"
