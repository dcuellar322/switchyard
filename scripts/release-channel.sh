#!/bin/sh
set -eu

tag=${1:-}
if ! printf '%s\n' "$tag" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(nightly|alpha|beta)\.[0-9A-Za-z][0-9A-Za-z.-]*)?$'; then
	printf 'invalid Switchyard release tag: %s\n' "$tag" >&2
	exit 2
fi

case "$tag" in
	*-nightly.*) channel=nightly ;;
	*-alpha.*) channel=alpha ;;
	*-beta.*) channel=beta ;;
	*) channel=stable ;;
esac

printf '%s\n' "$channel"
if [ -n "${GITHUB_OUTPUT:-}" ]; then
	printf 'tag=%s\nchannel=%s\n' "$tag" "$channel" >> "$GITHUB_OUTPUT"
fi
