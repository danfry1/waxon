Output demo/recordings/lyrics.gif
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

# Play a track so the lyrics clock starts from the top
Tab
Sleep 500ms
Enter
Sleep 1500ms

# Open Now Playing — full-screen album art
Type "N"
Sleep 2s

# Toggle time-synced lyrics with `l`
Type "l"
Sleep 1s

# Watch the active line light up and advance in time with playback
Sleep 12s

# Flip back to the album art, then close
Type "l"
Sleep 2s
Escape
Sleep 1s
