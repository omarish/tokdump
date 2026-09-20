#compdef tokdump
# zsh completion for tokdump

_arguments -s -S \
  '(-h --help)'{-h,--help}'[show help]' \
  '(-v --version)'{-v,--version}'[print version]' \
  '(-x --hex)'{-x,--hex}'[print token IDs in hexadecimal]' \
  '(-n --narrow)'{-n,--narrow}'[2 token IDs per row instead of 4]' \
  '--strict[fail on input that is not valid UTF-8]' \
  '*:file:_files'
