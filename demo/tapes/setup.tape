Output demo/recordings/setup.gif
Set Shell bash
Set FontFamily "JetBrainsMono Nerd Font Mono"
Set FontSize 14
Set Width 1100
Set Height 760
Set TypingSpeed 40ms
Set Theme "Ayu Mirage"

# The guided setup, against the demo build's stand-ins for the browser
# sign-in and librespot (see demo_setup.go).
Hide
Type "clear"
Enter
Show

Type "./waxon-demo demo setup"
Enter
Sleep 2.5s

# Welcome → which Spotify app
Enter
Sleep 2s

# Your own app: the steps, and a live check of the client ID
Enter
Sleep 2.5s
Type "0123456789abcdef0123456789abcdef"
Sleep 1.5s
Enter

# Approve in the browser (the stand-in approves after a moment)
Sleep 3.5s

# Where should music play? Right here in waxon.
Sleep 1.5s
Down
Sleep 1.5s
Enter
Sleep 2.5s

# Install librespot (the stand-in prints like Homebrew)
Enter
Sleep 3s

# Sign the player in, then done
Sleep 3s
Sleep 3s
Enter
Sleep 500ms
