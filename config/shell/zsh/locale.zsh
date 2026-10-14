# Locale — ensure UTF-8 is set even on minimal images (Docker, WSL)
# Sourced after P10k instant prompt — must not produce console output.
# en_US.UTF-8 where it has been generated, else C.UTF-8, which glibc ships
# built in, so a fresh machine gets UTF-8 without generating a locale as root.
if [[ -z "$LANG" || "$LANG" == "C" || "$LANG" == "POSIX" ]]; then
  if [[ ${(L)"$(locale -a 2>/dev/null)"} == *en_us.utf(-|)8* ]]; then
    export LANG='en_US.UTF-8'
  else
    export LANG='C.UTF-8'
  fi
fi
export LC_ALL="${LC_ALL:-$LANG}"
