# VHS Demo for Starmap CLI
# This tape demonstrates the key features of the starmap CLI tool

# Output configuration - SVG format for best quality and smaller file size
Output scripts/demo.svg

# Window
Set Width 1680
Set Height 1050

# Style
Set FontFamily 'JetBrains Mono'
Set Theme "Dracula"
Set Padding 10

# Margin
Set Margin 40
Set MarginFill "#6B50FF"

# Terminal
# Set WindowBar Colorful
Set WindowBar Rings
Set WindowBarTitle Terminal

# Set shell and typing behavior
# Set Shell "zsh"
Set TypingSpeed 50ms

# Hide initial setup
Hide

# Ctrl+L terminal and set up
Ctrl+L

# Show the terminal
Show

Type "# Starmap[⭐]🗺️ - A comprehensive AI model catalog system"
Enter
Sleep 4s
Ctrl+L

# Help command
Type "./starmap --help"
Enter
Sleep 10s
Ctrl+L

# List models with explanation pause
Type "# First, let's see all available models"
Enter
Sleep 3s
Type "./starmap models list | head -26"
Sleep 4s
Enter
Sleep 7s

# List providers
Ctrl+L
Type "# Next let's see available providers"
Enter
Sleep 3s
Type "starmap providers | head -26"
Sleep 4s
Enter
Sleep 7s

# List authors
Ctrl+L
Type "# And the model authors/creators"
Enter
Sleep 3s
Type "starmap authors | head -26"
Sleep 4s
Enter
Sleep 7s