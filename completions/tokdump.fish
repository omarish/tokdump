# fish completion for tokdump
complete -c tokdump -s h -l help -d 'Show help'
complete -c tokdump -s v -l version -d 'Print version'
complete -c tokdump -s x -l hex -d 'Print token IDs in hexadecimal'
complete -c tokdump -s n -l narrow -d '2 token IDs per row instead of 4'
complete -c tokdump -l strict -d 'Fail on input that is not valid UTF-8'
# remaining args: default path/file completion
