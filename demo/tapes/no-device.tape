Output demo/recordings/no-device.gif
Set Shell bash
Set FontFamily "JetBrainsMono Nerd Font Mono"
Set FontSize 13
Set Width 1400
Set Height 800
Set TypingSpeed 80ms
Set Theme "Ayu Mirage"

Hide
Type "WAXON_DEMO_SCENARIO=no-device ./waxon-demo demo"
Enter
Sleep 2s
Show

# Nothing is playing and no device is active. Try to play a track.
Type "l"
Sleep 500ms
Type "jj"
Sleep 500ms
Enter
Sleep 1.5s

# waxon notices there's no active device and asks which one to use
Type "j"
Sleep 700ms
Enter
Sleep 2.5s

# Playback started on the chosen device; controls now work normally
Type "n"
Sleep 2s
