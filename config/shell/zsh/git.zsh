# From a terminal, `git diff` and `git show` open in diffview.nvim, the way
# MANPAGER sends man pages to nvim. Anything diffview cannot show the same way
# (--stat, --word-diff, a blob, a root commit, a range to show, an empty diff)
# runs git as usual, as does `command git`. DF_GIT_NVIM=0 turns it off.
[[ -o interactive ]] || return
(( $+commands[nvim] )) || return

git() {
  local -a dv
  if [[ $DF_GIT_NVIM != 0 && -t 0 && -t 1 && -z $NVIM && ( $1 == diff || $1 == show ) ]] &&
    [[ -d ${XDG_DATA_HOME:-$HOME/.local/share}/nvim/lazy/diffview.nvim ]] &&
    dv=("${(@f)$(_df_diffview_args "$@")}"); then
    DF_DIFFVIEW=$1 DF_DIFFVIEW_ARGS=${(pj:\n:)dv} nvim
  else
    command git "$@"
  fi
}

# Prints diffview's arguments for a git diff or show command line, one per
# line, or fails when diffview would show something different.
_df_diffview_args() {
  local cmd=$1 a dashdash=0
  local -a revs paths opts
  shift
  command git rev-parse --git-dir >/dev/null 2>&1 || return 1
  while (( $# )); do
    a=$1
    shift
    if (( dashdash )); then
      paths+=$a
      continue
    fi
    case $a in
      --) dashdash=1 ;;
      --cached|--staged) [[ $cmd == diff ]] || return 1; opts+=$a ;;
      -*|*:*) return 1 ;;
      *)
        # Revisions come before paths, as in git.
        if (( ! $#paths )) && [[ -n $(command git rev-parse -q --revs-only "$a" 2>/dev/null) ]]; then
          revs+=$a
        else
          # git takes a bare path only when it exists, and errors otherwise.
          [[ -e $a ]] || return 1
          paths+=$a
        fi
        ;;
    esac
  done

  if [[ $cmd == show ]]; then
    (( $#revs <= 1 )) || return 1
    local sha
    sha=$(command git rev-parse -q --verify "${revs[1]:-HEAD}^{commit}") || return 1
    # diffview's history lists nothing for a root commit.
    command git rev-parse -q --verify "$sha^" >/dev/null || return 1
    print -rl -- --range=$sha -n1 $paths
    return
  fi

  case $#revs in
    0|1) ;;
    2) [[ $revs[1] != *..* && $revs[2] != *..* ]] || return 1
       revs=("$revs[1]..$revs[2]") ;;
    *) return 1 ;;
  esac
  # Nothing to show: let git print its nothing.
  command git diff --quiet $opts $revs -- $paths 2>/dev/null && return 1
  print -rl -- -uno $opts $revs -- $paths
}
