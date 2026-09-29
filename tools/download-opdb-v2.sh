#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  echo "Usage: $0 [output_dir]"
  echo "  output_dir  directory to store the downloaded file and .etag (default: current directory)"
  exit 0
fi

URL="https://mp-data.sfo3.cdn.digitaloceanspaces.com/opdb-v2.json"
OUTPUT_DIR="${1:-.}"
mkdir -p "$OUTPUT_DIR"
ETAG_FILE="$OUTPUT_DIR/.etag"
TIMESTAMP="$(date +%s)"

# Private temp files; the body is created in OUTPUT_DIR so the final mv is an
# atomic rename on the same filesystem. Both are removed on any exit.
HEADERS_FILE="$(mktemp)"
BODY_FILE="$(mktemp "$OUTPUT_DIR/.opdb_body.XXXXXX")"
trap 'rm -f "$HEADERS_FILE" "$BODY_FILE"' EXIT

# Build curl args, adding If-None-Match only if we have a cached etag AND the
# file it refers to is still on disk. Otherwise a 304 would leave us with no file.
CURL_ARGS=(-sS -D "$HEADERS_FILE" -o "$BODY_FILE" -w "%{http_code}")

if [[ -f "$ETAG_FILE" ]]; then
  CACHED_ETAG="$(cat "$ETAG_FILE")"
  if compgen -G "$OUTPUT_DIR/opdb-v2.*.${CACHED_ETAG}.json" > /dev/null; then
    echo "Found cached ETag: $CACHED_ETAG"
    CURL_ARGS+=(-H "If-None-Match: \"$CACHED_ETAG\"")
  else
    echo "Cached ETag $CACHED_ETAG has no matching file in $OUTPUT_DIR, doing a full download."
  fi
else
  echo "No cached ETag found, doing a full download."
fi

HTTP_CODE="$(curl "${CURL_ARGS[@]}" "$URL")"

case "$HTTP_CODE" in
  200)
    echo "New content received (HTTP 200)."

    # Extract ETag from response headers (strip quotes and CR)
    NEW_ETAG="$(grep -i '^etag:' "$HEADERS_FILE" | cut -d: -f2- | tr -d '"\r\n' | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"

    if [[ -z "$NEW_ETAG" ]]; then
      echo "Warning: server did not return an ETag header."
      NEW_ETAG="unknown"
    fi

    OUTPUT_FILE="$OUTPUT_DIR/opdb-v2.${TIMESTAMP}.${NEW_ETAG}.json"
    mv "$BODY_FILE" "$OUTPUT_FILE"
    echo "$NEW_ETAG" > "$ETAG_FILE"

    echo "Saved: $OUTPUT_FILE"
    echo "Updated $ETAG_FILE with new ETag."
    ;;
  304)
    echo "Not modified (HTTP 304) — remote content unchanged, skipping download."
    ;;
  *)
    echo "Unexpected HTTP status: $HTTP_CODE"
    cat "$HEADERS_FILE"
    exit 1
    ;;
esac
