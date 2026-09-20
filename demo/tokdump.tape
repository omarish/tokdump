# tokdump v0.3.0 demo still.
# Render: make demo  ->  demo/tokdump-v0.3.0.png
#
# Per knowledge-base/marketing/terminal-demos.md:
#   - Electric Sky over Obsidian; stock themes are an anti-pattern.
#   - Single-command beat, so full-bleed: no window chrome, no traffic lights.
#   - Fit the viewport to the content. The doc's anti-example is literally a
#     "printf | tokdump"-class command in a 1600x900 void, so the frame is
#     sized to hug the dump with ~40px padding instead.
#   - -n keeps the block from sprawling into a wide ribbon.
#   - The example shows both behaviours at once: the first emoji is split
#     across two tokens and dumps as raw bytes, the second survives as one
#     whole token and renders as itself.

Output demo/.scratch.mp4

Set Shell "bash"
Set FontSize 40
Set FontFamily "JetBrains Mono"
Set Width 1180
Set Height 360
Set Padding 40
Set Theme {"name":"Electric Sky","black":"#171717","red":"#F43F5E","green":"#34D399","yellow":"#FBBF24","blue":"#38BDF8","magenta":"#818CF8","cyan":"#38BDF8","white":"#E2E8F0","brightBlack":"#475569","brightRed":"#F43F5E","brightGreen":"#34D399","brightYellow":"#FBBF24","brightBlue":"#38BDF8","brightMagenta":"#818CF8","brightCyan":"#7DD3FC","brightWhite":"#F8FAFC","background":"#171717","foreground":"#C4C4C4","selection":"#1E293B","cursor":"#38BDF8"}

Hide
Type "clear" Enter
Show
Type "printf '💯🔥' | tokdump -n" Enter
Sleep 2s
