# phogo

**phogo** is a lightweight, terminal-native image gallery and ASCII renderer built with Go and the Bubble Tea framework. It transforms your images into vibrant ASCII art.

![phogo gif](assets/phogo.gif)

## Features

* **Smart Launch**: Open specific files or directories directly from the CLI.
* **Live ASCII Filters**: Swap between **Color**, **Grayscale**, **Inverted**, and **Duotone** modes instantly using `1-4`.
* **Slideshow Mode**: Hands-free viewing with an automatic looping slideshow.
* **Wayland Integration**: Native `wl-copy` support for copying file paths.
* **Headless Conversion**: A dedicated `--convert` flag to spit ASCII art directly to `stdout`.
* **File Management**: Rename, delete, and search for files without leaving the TUI.
* **Advanced Sorting**: Organize your gallery by Name, Size, or Date Modified.

---

## Installation

Download from [relaese page](https://github.com/SirSobhan0/phogo/releases)

||

```bash
go install github.com/SirSobhan0/phogo@latest
```

||

Compile from source:

Ensure you have [Go](https://go.dev/) installed, then:

```bash
# Clone the repository
git clone https://github.com/SirSobhan0/phogo.git
cd phogo

# Install dependencies
go mod download

# Build and install the binary
go build -o phogo main.go
sudo mv phogo /usr/local/bin/
```
Note: For clipboard support on Wayland, ensure wl-clipboard is installed.

## Usage

```bash
# Open phogo in the current directory
phogo

# Open phogo in a specific directory
phogo ~/Pictures/Wallpapers

# Open a specific image directly
phogo drone_shot.jpg

# Convert an image to ASCII and output to terminal (Headless)
phogo --convert drone_shot.jpg
```

## Keybinds

### Browsing Mode
|Key|	Action|
|-|-|
j / k|	Move cursor down/up
Enter|	View Image / Open Folder
P|	Start Looping Slideshow
s|	Cycle Sort (Name, Size, Date)
h|	Toggle Hidden Files
/|	Filter/Search files
r|	Rename selected file
x|	Delete selected file
y|	Copy absolute path to clipboard (Wayland)
d|	Set current directory (in Folder Browser)
q|	Quit

### Viewer Mode
|Key|	Action|
|-|-|
1|	Color Filter
2|	Grayscale Filter
3|	Inverted Filter
4|	Duotone Filter
Esc / q|	Back to browser
Any Key|	Stop Slideshow

## License

This project is licensed under the GPL-3.0-or-later License, see the `COPYING` file for details.
