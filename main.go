package main

import (
	"fmt"
	_ "image/jpeg" // Register JPEG decoder
	_ "image/png"  // Register PNG decoder
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput" // Re-introduced for custom search
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
	stateDirBrowsing 
	stateSearching   // Re-introduced state for dedicated search input
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
	
	dirStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#99CCFF"))
	searchStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC66"))
)

// --- Custom Items for the File List ---

type item struct {
	title, desc string
	fileName    string
	isDir       bool
}

func (i item) Title() string { 
    if i.isDir {
        return dirStyle.Render("📁 " + i.title)
    }
    return i.title
}
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title } 

// --- Main Model ---

type model struct {
	state        sessionState
	list         list.Model
	viewport     viewport.Model
	searchInput  textinput.Model // Re-introduced
	currentDir   string 
	dirBrowserPath string
	searchQuery  string // Re-introduced
	showHidden   bool   
	statusMsg    string
	imgContent   string 
	prevDirState sessionState // Re-introduced
}

func initialModel() model {
	currentDir, _ := os.Getwd()
    
	// 1. Setup File List 
	l := list.New(getFiles(currentDir, false, "", false), list.NewDefaultDelegate(), 0, 0)
	l.Title = "Local Photo Viewer"
	l.SetShowStatusBar(false)
	
	// CRITICAL FIX: Disable built-in filter AND clear its keybinding to remove the help text.
	l.KeyMap.Filter.SetEnabled(false)
	l.KeyMap.Filter.SetKeys() 
	l.KeyMap.Quit.SetKeys("q", "ctrl+c")


	// 2. Setup Search Input
	ti := textinput.New()
	ti.Placeholder = "Search files or folders..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 30
	
	// 3. Setup Viewport 
	vp := viewport.New(0, 0)

	return model{
		state:        stateBrowsing,
		list:         l,
		viewport:     vp,
		searchInput:  ti,
		currentDir:   currentDir,
		dirBrowserPath: currentDir,
		searchQuery:  "",
		showHidden:   false, 
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

// --- Directory Setting Logic ---

func (m *model) finalizeDirectoryChange(newPath string) {
	m.currentDir = newPath
	// Keep search/hidden status active when changing dir for continuity
	m.reloadImageList()
	m.list.Title = "Local Photo Viewer (Dir: " + m.currentDir + ")"
	m.statusMsg = "Working directory set to: " + m.currentDir
	m.state = stateBrowsing
}

// --- List Reload Helper ---

func (m *model) reloadImageList() {
	m.list.SetItems(getFiles(m.currentDir, false, m.searchQuery, m.showHidden))
}

func (m *model) reloadDirList() {
	m.list.SetItems(getFiles(m.dirBrowserPath, true, m.searchQuery, m.showHidden))
	m.list.Title = "Select Directory (Path: " + m.dirBrowserPath + ")"
}


// --- Update Loop ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		if m.state == stateSearching {
			m.list.SetSize(msg.Width-h, msg.Height-v-4)
		} else {
			m.list.SetSize(msg.Width-h, msg.Height-v)
		}
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3

	case tea.KeyMsg:
		// Global quit/back (q/ctrl+c) logic remains
		if msg.String() == "ctrl+c" || (msg.String() == "q" && m.state != stateSearching) {
			if m.state != stateBrowsing {
				m.state = stateBrowsing
				m.statusMsg = ""
				return m, nil
			}
			return m, tea.Quit
		}

		switch m.state {

		// 1. IMAGE BROWSING MODE
		case stateBrowsing:
			switch msg.String() {
			case "enter":
				selectedItem := m.list.SelectedItem()
				if selectedItem != nil {
					itm := selectedItem.(item)
					filePath := filepath.Join(m.currentDir, itm.fileName) 
					
					str, err := renderImage(filePath, m.viewport.Width, m.viewport.Height)
					if err != nil {
						m.statusMsg = "Error: " + err.Error()
					} else {
						m.imgContent = str
						m.viewport.SetContent(str)
						m.viewport.GotoTop()
						m.state = stateViewingImage
					}
				}
			case "d": // 'd' to enter directory browsing
				m.state = stateDirBrowsing
				m.reloadDirList()
				return m, nil
			case "/": // '/' to start searching (Custom mapping)
				m.prevDirState = stateBrowsing
				m.state = stateSearching
				m.searchInput.SetValue(m.searchQuery)
				m.searchInput.Focus()
				return m, textinput.Blink
			case "h": // 'h' to toggle hidden files
				m.showHidden = !m.showHidden
				m.reloadImageList()
				m.statusMsg = fmt.Sprintf("Show Hidden Files: %t", m.showHidden)
			}
			m.list, cmd = m.list.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}


		// 2. IMAGE VIEWER MODE (Unchanged)
		case stateViewingImage:
			switch msg.String() {
			case "esc":
				m.state = stateBrowsing
			}
			m.viewport, cmd = m.viewport.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
            
		// 3. DIRECTORY BROWSING MODE
		case stateDirBrowsing:
			switch msg.String() {
			case "enter":
				selectedItem := m.list.SelectedItem()
				if selectedItem != nil {
					itm := selectedItem.(item)
					
					newPath := ""
					if itm.fileName == ".." {
						newPath = filepath.Dir(m.dirBrowserPath)
					} else {
						newPath = filepath.Join(m.dirBrowserPath, itm.fileName)
					}
					
					info, err := os.Stat(newPath)
					if err == nil && info.IsDir() {
						m.dirBrowserPath = newPath 
						m.reloadDirList() 
					}
				}
			case "d", "esc":
				m.finalizeDirectoryChange(m.dirBrowserPath)
				return m, nil
			case "/": // '/' to start searching from dir mode (Custom mapping)
				m.prevDirState = stateDirBrowsing
				m.state = stateSearching
				m.searchInput.SetValue(m.searchQuery)
				m.searchInput.Focus()
				return m, textinput.Blink
			case "h": // 'h' to toggle hidden files in dir mode
				m.showHidden = !m.showHidden
				m.reloadDirList()
				m.statusMsg = fmt.Sprintf("Show Hidden Files: %t", m.showHidden)
			}
			
			m.list, cmd = m.list.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		// 4. SEARCHING MODE
		case stateSearching:
			switch msg.String() {
			case "enter":
				m.searchQuery = m.searchInput.Value()
				m.state = m.prevDirState 
				
				if m.state == stateDirBrowsing {
					m.reloadDirList()
					m.statusMsg = fmt.Sprintf("Search applied to directories: \"%s\"", m.searchQuery)
				} else {
					m.reloadImageList()
					m.statusMsg = fmt.Sprintf("Search applied to images: \"%s\"", m.searchQuery)
				}
				m.list.SetSize(m.viewport.Width, m.viewport.Height) 
				return m, nil
			case "esc":
				m.state = m.prevDirState 
				m.list.SetSize(m.viewport.Width, m.viewport.Height) 
				return m, nil
			}
			
			m.searchInput, cmd = m.searchInput.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// --- View Rendering ---

func (m model) View() string {
	var hint string
	var hiddenStatus string
	
	if m.searchQuery != "" {
		hint = searchStyle.Render(fmt.Sprintf(" [Active Search: \"%s\"]", m.searchQuery))
	}
	if m.showHidden {
		hiddenStatus = " (H: ON)"
	}

	switch m.state {
	case stateBrowsing:
		return appStyle.Render(m.list.View() + "\n" + statusMessageStyle(m.statusMsg) + 
			fmt.Sprintf("\n(Press 'd' to change dir, '/' to search, 'h' to toggle hidden)%s%s", hiddenStatus, hint))

	case stateViewingImage:
		return fmt.Sprintf("%s\n%s\n%s",
			titleStyle.Render("Image Viewer (Press 'q' or 'esc' to back)"),
			m.viewport.View(),
			statusMessageStyle(m.statusMsg),
		)
        
	case stateDirBrowsing:
		return appStyle.Render(
			fmt.Sprintf("%s\n%s",
				m.list.View(),
				statusMessageStyle("Navigate to folder and press 'd' or 'esc' to set it.") + 
				fmt.Sprintf("\n(Enter to navigate, '/' to search, 'h' to toggle hidden)%s%s", hiddenStatus, hint),
			),
		)
	
	case stateSearching:
		searchScope := "Images"
		if m.prevDirState == stateDirBrowsing {
			searchScope = "Directories"
		}

		inputContent := m.searchInput.View()
		
		return appStyle.Render(fmt.Sprintf(
			"Search %s (Press Enter to apply, Esc to cancel):\n\n%s\n%s",
			searchScope,
			inputContent,
			m.list.View(),
		))
	}
	return ""
}

// --- Helpers ---

// getFiles lists files or directories in the given path, applying filters.
func getFiles(dir string, dirsOnly bool, searchQuery string, showHidden bool) []list.Item {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []list.Item{item{title: "Error reading directory", desc: err.Error()}}
	}
    
	var items []list.Item
	query := strings.ToLower(searchQuery)

	if dirsOnly {
		absPath, _ := filepath.Abs(dir)
		parentDir := filepath.Dir(absPath)
		if absPath != parentDir { 
			items = append(items, item{
				title:    "..",
				desc:     "Go up one directory",
				fileName: "..",
				isDir:    true,
			})
		}
	}

	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
        
		name := e.Name()
		
		// 1. Hidden File Filter
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
        
		if e.IsDir() {
			if dirsOnly {
				// 2. Search Query Filter for Directories
				if searchQuery == "" || strings.Contains(strings.ToLower(name), query) {
					items = append(items, item{
						title:    name,
						desc:     "Directory",
						fileName: name,
						isDir:    true,
					})
				}
			}
		} else if !dirsOnly {
			// 3. Image File Type Filter
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				
				// 4. Search Query Filter for Image Files
				if searchQuery == "" || strings.Contains(strings.ToLower(name), query) {
					items = append(items, item{
						title:    name,
						desc:     fmt.Sprintf("%d bytes", info.Size()),
						fileName: name,
						isDir:    false,
					})
				}
			}
		}
	}
    
	// Sort results alphabetically
	sort.Slice(items, func(i, j int) bool {
		itemI := items[i].(item)
		itemJ := items[j].(item)

		if dirsOnly {
			if itemI.title == ".." { return true }
			if itemJ.title == ".." { return false }
		}
        
		return itemI.title < itemJ.title
	})

	return items
}

// renderImage converts a local image file to an ASCII string
func renderImage(path string, w, h int) (string, error) {
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
