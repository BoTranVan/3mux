`3mux` is a terminal multiplexer with out-of-the-box support for search, mouse-controlled scrollback, and i3-like keybindings. Imagine `tmux` with a smaller learning curve and more sane defaults.

<img src="./demo.gif" width="800"/>

<!--TODO: GIF!-->

### Features

* i3-like keybindings
* search
* scrollback
* mouse support
  * drag to resize windows
  * click to select window
  * scrollwheel

### Key Bindings

| Key(s) | Description
|-------:|:------------
|<kbd>Alt+Enter</kbd><br><kbd>Alt+N</kbd> | Create a new window
|<kbd>Alt+F</kbd> | Make the selected window fullscreen. Useful for copying text
|<kbd>Alt+&larr;/&darr;/&uarr;/&rarr;</kbd><br><kbd>Alt+h/j/k/l</kbd> | Select an adjacent window
|<kbd>Alt+Shift+&larr;/&darr;/&uarr;/&rarr;</kbd><br><kbd>Alt+Shift+h/j/k/l</kbd> | Move the selected window
|<kbd>Alt+R</kbd> | Enter resize mode. Resize selected window with arrow keys or <kbd>h/j/k/l</kbd>. Exit using any other key(s)
|<kbd>Alt+/</kbd> | Enter search mode. Type query, navigate between results with arrow keys or <kbd>n/N</kbd>
|<kbd>Scroll</kbd> | Move through scrollback

### Installation Instructions

1. Install Golang
2. `go get github.com/aaronjanse/3mux`
3. Run `3mux` to launch the terminal multiplexer

#### Terminal.app
Preferences > Profiles > Keyboard > Use Option as Meta Key

#### iTerm2
Preferences > Profiles > Keys > Option Key > Esc+
