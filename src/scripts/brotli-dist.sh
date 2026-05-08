#!/usr/bin/env bash

set -euo pipefail

target_dir="${1:-dist}"

if ! command -v brotli >/dev/null 2>&1; then
  echo "brotli is required but was not found in PATH" >&2
  exit 1
fi

if ! command -v gzip >/dev/null 2>&1; then
  echo "gzip is required but was not found in PATH" >&2
  exit 1
fi

if [[ ! -d "$target_dir" ]]; then
  echo "Directory not found: $target_dir" >&2
  exit 1
fi

format_bytes() {
  local bytes="$1"
  awk -v bytes="$bytes" '
    function abs(v) { return v < 0 ? -v : v }
    BEGIN {
      split("B KiB MiB GiB TiB", units, " ")
      value = bytes + 0
      unit = 1
      while (abs(value) >= 1024 && unit < 5) {
        value /= 1024
        unit++
      }
      printf "%.2f %s", value, units[unit]
    }
  '
}

print_summary_line() {
  local path="$1"
  local before="$2"
  local after="$3"
  local diff=$((before - after))
  local percent

  if (( before == 0 )); then
    percent="0.00"
  else
    percent="$(awk -v before="$before" -v diff="$diff" 'BEGIN { printf "%.2f", (diff / before) * 100 }')"
  fi

  printf '%s: %s -> %s (%s, %s%%)\n' \
    "$path" \
    "$(format_bytes "$before")" \
    "$(format_bytes "$after")" \
    "$(format_bytes "$diff")" \
    "$percent"
}

total_before=0
total_br_after=0
total_gz_after=0
file_count=0

while IFS= read -r -d '' file; do
  before_size="$(stat -c %s "$file")"

  # Brotli sidecar (.br) — original file is kept untouched
  brotli --force --quality=11 --output="${file}.br" "$file"
  br_size="$(stat -c %s "${file}.br")"

  # Gzip sidecar (.gz) — keep original, no timestamp in header
  gzip --best --keep --force --no-name "$file"
  gz_size="$(stat -c %s "${file}.gz")"

  total_before=$((total_before + before_size))
  total_br_after=$((total_br_after + br_size))
  total_gz_after=$((total_gz_after + gz_size))
  file_count=$((file_count + 1))

  printf '%s (%s)\n' "$file" "$(format_bytes "$before_size")"
  printf '  br:   '; print_summary_line "  " "$before_size" "$br_size"
  printf '  gz:   '; print_summary_line "  " "$before_size" "$gz_size"
done < <(
  find "$target_dir" -type f \
    \( -iname '*.html' -o -iname '*.css' -o -iname '*.js' \) \
    ! -iname '*.br' ! -iname '*.gz' \
    -print0 | sort -z
)

if (( file_count == 0 )); then
  echo "No files found in $target_dir"
  exit 0
fi

echo
echo "Processed $file_count files"
printf 'Total plain: %s\n' "$(format_bytes "$total_before")"
printf 'Total br:    '; print_summary_line "Total" "$total_before" "$total_br_after"
printf 'Total gz:    '; print_summary_line "Total" "$total_before" "$total_gz_after"
