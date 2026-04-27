# Locale — ensure UTF-8 is set even on minimal images (Docker, WSL)
# Sourced after P10k instant prompt — must not produce console output.
if [[ -z "$LANG" || "$LANG" == "C" || "$LANG" == "POSIX" ]]; then
  export LANG='en_US.UTF-8'
fi
export LC_ALL="${LC_ALL:-$LANG}"
