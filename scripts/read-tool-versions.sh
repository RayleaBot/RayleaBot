#!/usr/bin/env bash
# Bootstrap version setup using only Bash, before a language runtime is selected.
set -euo pipefail
seen=' '
output=''
while IFS= read -r line || [[ -n "$line" ]]; do
  line="${line%$'\r'}"
  line="${line%%#*}"
  read -r tool version extra <<< "$line"
  [[ -z "${tool:-}" ]] && continue
  if [[ -n "${extra:-}" || ! "${tool:-}" =~ ^[a-z]+$ || ! "${version:-}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo 'Invalid fixed toolchain declaration' >&2
    exit 1
  fi
  case "$seen" in *" $tool "*) echo "Duplicate tool: $tool" >&2; exit 1;; esac
  seen="$seen$tool "
  output="$output$tool=$version"$'\n'
done < "${1:-.tool-versions}"
for tool in golang nodejs python pnpm npm corepack sqlc; do
  case "$seen" in *" $tool "*) ;; *) echo "Missing tool: $tool" >&2; exit 1;; esac
done
printf '%s' "$output"
