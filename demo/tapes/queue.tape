Output demo/recordings/queue.gif
Set Shell bash
Set FontFamily "JetBrainsMono Nerd Font Mono"
Set FontSize 13
Set Width 1400
Set Height 800
Set TypingSpeed 80ms
Set Theme "Ayu Mirage"

Hide
Type "./waxon-demo demo"
Enter
Sleep 2s
Show

# Play a playlist, then queue two tracks from further down
Type "jj"
Sleep 600ms
Enter
Sleep 1.5s
Enter
Sleep 1.5s
Type "5j"
Sleep 500ms
Type "a"
Sleep 800ms
Type "j"
Sleep 400ms
Type "a"
Sleep 1s

# The queue: what you added, then what plays next from the playlist
Type "gq"
Sleep 2.5s
Type "jjj"
Sleep 800ms
Type "jj"
Sleep 800ms

# Skip forward to a track; the rest of the queue stays
Enter
Sleep 3s
