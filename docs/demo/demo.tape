# Records docs/demo/demo.gif and docs/demo/screenshot.png with VHS
# (https://github.com/charmbracelet/vhs). Run `make demo` from the repo root.

Output docs/demo/demo.gif
Set Shell bash
Set FontFamily "DejaVu Sans Mono"
Set FontSize 16
Set LineHeight 1.0
Set Width 1200
Set Height 640
Set Padding 20
Set Theme "GitHub Dark"
Set TypingSpeed 80ms
Env COLORTERM "truecolor"
Hide
Type "export HMM=$PWD/bin/hmm && cd $(mktemp -d) && cp $OLDPWD/docs/demo/trip.opml . && clear && $HMM trip.opml" Enter
Sleep 1s
Show
Sleep 1.5s
# wander around the map
Type "l" Sleep 500ms
Type "l" Sleep 500ms
Type "j" Sleep 500ms
# add a sibling and type it in
Enter Sleep 400ms
Type "Matcha in Uji" Sleep 400ms
Escape Sleep 800ms
# reorder it up among its siblings
Type "K" Sleep 500ms
Type "K" Sleep 800ms
# jump over to the left side of the map
Type "h" Sleep 400ms
Type "h" Sleep 400ms
Type "h" Sleep 500ms
Type "k" Sleep 500ms
# add a child to Kyoto
Tab Sleep 400ms
Type "Nishiki market" Sleep 400ms
Escape Sleep 1s
# peek at the full help
Type "?" Sleep 2s
Type "?" Sleep 1s
Screenshot docs/demo/screenshot.png
Sleep 500ms
