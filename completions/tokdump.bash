# bash completion for tokdump
_tokdump() {
  local cur prev
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  case "${cur}" in
    -*)
      COMPREPLY=( $(compgen -W "-h --help -v --version -x --hex --strict" -- "${cur}") )
      return 0
      ;;
  esac

  COMPREPLY=( $(compgen -f -- "${cur}") )
  return 0
}
complete -F _tokdump tokdump
