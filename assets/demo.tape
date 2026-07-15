# driftcheck hero demo — regenerate with: cd examples && vhs ../assets/demo.tape
# (vhs: brew install vhs)

Output ../assets/demo.gif

Set Theme "Catppuccin Mocha"
Set FontSize 15
Set Width 1000
Set Height 540
Set Padding 16
Set TypingSpeed 35ms

Sleep 500ms

Type "# same config, two environments — what actually differs?"
Enter
Sleep 800ms

Type "driftcheck compare dev.env prod.env"
Enter
Sleep 4.5s

Type "clear"
Enter
Sleep 300ms

Type `# type-aware: catches "8080" (string) vs 8080 (int)`
Enter
Sleep 800ms

Type "driftcheck compare app-dev.yaml app-prod.yaml"
Enter
Sleep 4s

Type "echo $?   # exit 1 = drift found: gate your CI on it"
Enter
Sleep 3.5s
