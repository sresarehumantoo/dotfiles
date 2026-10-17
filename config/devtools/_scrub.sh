# shellcheck shell=bash
# Sourced by the git and gh wrappers: the one definition of AI attribution,
# whether it sits in a commit message, a PR body or a comment.

_scrub_ai='claude|anthropic|copilot|openai|chatgpt|codex|cursor|gemini|aider|devin|windsurf|codeium'

# Each predicate is anchored at the start of a line, so a line of JSON is
# never mistaken for one. \uXXXX allows for a JSON-escaped robot emoji.
_scrub_awk='
function attribution(s,   l) {
    l = tolower(s)
    if (l ~ /^[ \t]*co-authored-by:/ && l ~ ai) return 1
    if (l ~ /^([^a-z0-9]|\\u[0-9a-f][0-9a-f][0-9a-f][0-9a-f])*generated (with|by|using) / && l ~ ai) return 1
    if (l ~ /^[ \t]*claude-session:/) return 1
    return l ~ /^[^a-z0-9]*https?:\/\/claude\.ai\/code\/session/
}'

# Drops attribution lines from stdin, then any trailing blank lines.
scrub() {
    awk -v ai="$_scrub_ai" "$_scrub_awk"'
        attribution($0) { next }
        { buf[++n] = $0 }
        END {
            while (n > 0 && buf[n] ~ /^[ \t]*$/) n--
            for (i = 1; i <= n; i++) print buf[i]
        }'
}

# The same for JSON or GraphQL source, where a line break inside a string is
# the two characters \n: an attribution segment is cut up to the closing quote
# of its string, along with the blank lines before it.
scrub_escaped() {
    awk -v ai="$_scrub_ai" "$_scrub_awk"'
        {
            n = split($0, seg, /\\n/)
            out = seg[1]
            for (i = 2; i <= n; i++) {
                if (!attribution(seg[i])) {
                    out = out "\\n" seg[i]
                    continue
                }
                if (match(seg[i], /[^\\]"/)) {
                    sub(/(\\n)+$/, "", out)
                    out = out substr(seg[i], RSTART + 1)
                }
            }
            print out
        }'
}
