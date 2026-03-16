#!/usr/bin/env bash

set -euo pipefail

target_dir="${1:-dist}"

if ! command -v brotli >/dev/null 2>&1; then
  echo "brotli is required but was not found in PATH" >&2
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
total_after=0
file_count=0

while IFS= read -r -d '' file; do
  before_size="$(stat -c %s "$file")"
  temp_file="$(mktemp)"

  brotli --force --quality=11 --output="$temp_file" "$file"
  mv "$temp_file" "$file"

  after_size="$(stat -c %s "$file")"

  total_before=$((total_before + before_size))
  total_after=$((total_after + after_size))
  file_count=$((file_count + 1))

  print_summary_line "$file" "$before_size" "$after_size"
done < <(
  find "$target_dir" -type f \
    \( -iname '*.html' -o -iname '*.css' -o -iname '*.js' \) \
    -print0 | sort -z
)

if (( file_count == 0 )); then
  echo "No files found in $target_dir"
  exit 0
fi

echo
echo "Processed $file_count files"
print_summary_line "Total" "$total_before" "$total_after"
