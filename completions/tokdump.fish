# fish completion for tokdump
complete -c tokdump -s h -l help -d 'Show help'
complete -c tokdump -s v -l version -d 'Print version'
complete -c tokdump -s x -l hex -d 'Print token IDs in hexadecimal'
complete -c tokdump -l strict -d 'Fail on input that is not valid UTF-8'
# remaining args: default path/file completion
