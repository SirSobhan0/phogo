package main

import (
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/qeesung/image2ascii/convert"
)

// --- Enums & Styles ---

type sessionState int

const (
	stateBrowsing sessionState = iota
	stateViewingImage
)

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A0A0A0")).
				Render
)

// --- Custom Items for the File List ---

type item struct {
	title, desc string
	fileName    string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

// --- Main Model ---

type model struct {
	state      sessionState
	list       list.Model
	viewport   viewport.Model
	statusMsg  string
	imgContent string // The ASCII string of the image
}

func initialModel() model {
	// 1. Setup File List
	l := list.New(getFiles("."), list.NewDefaultDelegate(), 0, 0)
	l.Title = "Local Photo Viewer"
	l.SetShowStatusBar(false)

	// 2. Setup Viewport (for Image)
	vp := viewport.New(0, 0)

	return model{
		state:    stateBrowsing,
		list:     l,
		viewport: vp,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

// --- Update Loop ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// Window Resize Event
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3 

	// Key Press Events
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.state != stateBrowsing {
				m.state = stateBrowsing
				m.statusMsg = ""
				return m, nil
			}
			return m, tea.Quit
		}

		switch m.state {

		// 1. BROWSING FILE LIST
		case stateBrowsing:
			switch msg.String() {
			case "enter":
				selectedItem := m.list.SelectedItem()
				if selectedItem != nil {
					itm := selectedItem.(item)
					// Render image to ASCII
					str, err := renderImage(itm.fileName, m.viewport.Width, m.viewport.Height)
					if err != nil {
						m.statusMsg = "Error: " + err.Error()
					} else {
						m.imgContent = str
						m.viewport.SetContent(str)
						m.state = stateViewingImage
					}
				}
			}
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)

		// 2. VIEWING IMAGE
		case stateViewingImage:
			switch msg.String() {
			case "esc":
				m.state = stateBrowsing
			}
			// Viewport handles scrolling with keys like Up/Down, PgUp/PgDown
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// --- View Rendering ---

func (m model) View() string {
	switch m.state {
	case stateBrowsing:
		return appStyle.Render(m.list.View() + "\n" + statusMessageStyle(m.statusMsg))

	case stateViewingImage:
		return fmt.Sprintf("%s\n%s\n%s",
			titleStyle.Render("Image Viewer (Press 'q' or 'esc' to back)"),
			m.viewport.View(),
			statusMessageStyle(m.statusMsg),
		)
	}
	return ""
}

// --- Helpers ---

// getFiles lists all images in current directory
func getFiles(dir string) []list.Item {
	entries, _ := os.ReadDir(dir)
	var items []list.Item
	for _, e := range entries {
		if !e.IsDir() {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				info, _ := e.Info()
				items = append(items, item{
					title:    e.Name(),
					desc:     fmt.Sprintf("%d bytes", info.Size()),
					fileName: e.Name(),
				})
			}
		}
	}
	return items
}

// renderImage converts a local image file to an ASCII string
// This function uses the corrected signature based on our previous debugging.
func renderImage(path string, w, h int) (string, error) {
	// Create convert options
	convertOptions := convert.DefaultOptions
	convertOptions.FixedWidth = w
	convertOptions.FixedHeight = h
	convertOptions.Colored = true

	converter := convert.NewImageConverter()

	return converter.ImageFile2ASCIIString(path, &convertOptions), nil
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
