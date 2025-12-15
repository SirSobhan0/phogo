package main

import (
	"fmt"
	_ "image/jpeg" 
	_ "image/png"  
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput" 
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/qeesung/image2ascii/convert"
)

type sessionState int

const (
	stateBrowsing sessionState = iota 
	stateViewingImage
	stateDirBrowsing 
	stateSearching   
	stateFilterSelection 
)

const (
	FilterColor    = "Color"
	FilterGrayscale = "Grayscale"
	FilterInverted = "Inverted"
	FilterDuotone  = "Duotone"
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
	filterTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF88FF"))
)

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

type filterItem struct {
	id string
	desc string
}

func (f filterItem) Title() string { return f.id }
func (f filterItem) Description() string { return f.desc }
func (f filterItem) FilterValue() string { return f.id }


type model struct {
	state          sessionState
	list           list.Model
	viewport       viewport.Model
	searchInput    textinput.Model 
	currentDir     string 
	dirBrowserPath string
	searchQuery    string 
	showHidden     bool   
	statusMsg      string
	imgContent     string 
	prevDirState   sessionState 

	filterMode     string 
	filterList     list.Model 
}

func initialModel() model {
	currentDir, _ := os.Getwd()
    
	l := list.New(getFiles(currentDir, false, "", false), list.NewDefaultDelegate(), 0, 0)
	l.Title = "Local Photo Viewer"
	l.SetShowStatusBar(false)
	l.KeyMap.Filter.SetEnabled(false)
	l.KeyMap.Filter.SetKeys() 
	l.KeyMap.Quit.SetKeys("q", "ctrl+c")

	ti := textinput.New()
	ti.Placeholder = "Search files or folders..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 30
	
	vp := viewport.New(0, 0)

	fList := list.New([]list.Item{
		filterItem{id: FilterColor, desc: "Renders the photo in full, true color."},
		filterItem{id: FilterInverted, desc: "Renders the photo with inverted colors (negative effect)."},
		filterItem{id: FilterGrayscale, desc: "Renders the photo in smooth shades of gray (monochrome)."},
		filterItem{id: FilterDuotone, desc: "Renders the photo in high-contrast black and white."},
	}, list.NewDefaultDelegate(), 0, 0)
	fList.Title = filterTitleStyle.Render("Select Image Filter (Press Enter)")
	fList.SetShowFilter(false)
	fList.SetShowStatusBar(false)
	
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.Foreground(lipgloss.Color("#FFCC66"))
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.Foreground(lipgloss.Color("#FFCC66"))
	fList.SetDelegate(d)

	m := model{
		state:        stateBrowsing,
		list:         l,
		viewport:     vp,
		searchInput:  ti,
		currentDir:   currentDir,
		dirBrowserPath: currentDir,
		searchQuery:  "",
		showHidden:   false,
		filterMode:   FilterColor, 
		filterList:   fList,
	}

	m.filterList.SetSize(40, 10)
    
    return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m *model) finalizeDirectoryChange(newPath string) {
	m.currentDir = newPath
	m.reloadImageList()
	m.list.Title = "Local Photo Viewer (Dir: " + m.currentDir + ")"
	m.statusMsg = "Working directory set to: " + m.currentDir
	m.state = stateBrowsing
}

func (m *model) reloadImageList() {
	m.list.SetItems(getFiles(m.currentDir, false, m.searchQuery, m.showHidden))
}

func (m *model) reloadDirList() {
	m.list.SetItems(getFiles(m.dirBrowserPath, true, m.searchQuery, m.showHidden))
	m.list.Title = "Select Directory (Path: " + m.dirBrowserPath + ")"
}


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
		m.filterList.SetSize(msg.Width/2, msg.Height - 10)


	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || (msg.String() == "q" && m.state != stateSearching) {
			if m.state != stateBrowsing {
				m.state = stateBrowsing
				m.statusMsg = ""
				return m, nil
			}
			return m, tea.Quit
		}

		switch m.state {

		case stateBrowsing:
			switch msg.String() {
			case "enter":
				selectedItem := m.list.SelectedItem()
				if selectedItem != nil {
					itm := selectedItem.(item)
					filePath := filepath.Join(m.currentDir, itm.fileName) 
					
					str, err := renderImage(filePath, m.viewport.Width, m.viewport.Height, m.filterMode)
					if err != nil {
						m.statusMsg = "Error: " + err.Error()
					} else {
						m.imgContent = str
						m.viewport.SetContent(str)
						m.viewport.GotoTop()
						m.state = stateViewingImage
					}
				}
			case "d": 
				m.state = stateDirBrowsing
				m.reloadDirList()
				return m, nil
			case "/": 
				m.prevDirState = stateBrowsing
				m.state = stateSearching
				m.searchInput.SetValue(m.searchQuery)
				m.searchInput.Focus()
				return m, textinput.Blink
			case "h": 
				m.showHidden = !m.showHidden
				m.reloadImageList()
				m.statusMsg = fmt.Sprintf("Show Hidden Files: %t", m.showHidden)
			case "f": 
				m.state = stateFilterSelection
				m.filterList.Select(findItemIndex(m.filterList.Items(), m.filterMode))
				return m, nil
			}
			m.list, cmd = m.list.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}


		case stateViewingImage:
			switch msg.String() {
			case "esc":
				m.state = stateBrowsing
			}
			m.viewport, cmd = m.viewport.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
            
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
			case "/": 
				m.prevDirState = stateDirBrowsing
				m.state = stateSearching
				m.searchInput.SetValue(m.searchQuery)
				m.searchInput.Focus()
				return m, textinput.Blink
			case "h": 
				m.showHidden = !m.showHidden
				m.reloadDirList()
				m.statusMsg = fmt.Sprintf("Show Hidden Files: %t", m.showHidden)
			}
			
			m.list, cmd = m.list.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

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
		
		case stateFilterSelection:
			switch msg.String() {
			case "enter":
				if f, ok := m.filterList.SelectedItem().(filterItem); ok {
					m.filterMode = f.id
				}
				m.statusMsg = fmt.Sprintf("Filter: %s applied.", m.filterMode)
				m.state = stateBrowsing
				return m, nil
			case "esc":
				m.state = stateBrowsing
				return m, nil
			}
			
			m.filterList, cmd = m.filterList.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	var hint string
	var hiddenStatus string
	
	if m.searchQuery != "" {
		hint = searchStyle.Render(fmt.Sprintf(" [S: \"%s\"]", m.searchQuery))
	}
	if m.showHidden {
		hiddenStatus = " (H: ON)"
	}
	
	filterStatus := fmt.Sprintf(" [F: %s]", m.filterMode)


	switch m.state {
	case stateBrowsing:
		return appStyle.Render(m.list.View() + "\n" + statusMessageStyle(m.statusMsg) + 
			fmt.Sprintf("\n(Press 'd' dir, '/' search, 'h' hidden, 'f' filter)%s%s%s", hiddenStatus, hint, filterStatus))

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
	
	case stateFilterSelection:
		msg := "Select an Image Filter. Press Enter to apply or ESC to cancel."
		
		return appStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
			msg,
			m.filterList.View(),
		))
	}
	return ""
}

func findItemIndex(items []list.Item, id string) int {
	for i, item := range items {
		if f, ok := item.(filterItem); ok && f.id == id {
			return i
		}
	}
	return 0
}


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
		
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
        
		if e.IsDir() {
			if dirsOnly {
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
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				
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
    
	return items
}

func renderImage(path string, w, h int, filterMode string) (string, error) {
	convertOptions := convert.DefaultOptions
	convertOptions.FixedWidth = w
	convertOptions.FixedHeight = h
    
    convertOptions.Colored = true
	convertOptions.Reversed = false
    
    converter := convert.NewImageConverter()

	switch filterMode {
	case FilterColor:
		convertOptions.Colored = true 
		convertOptions.Reversed = false

	case FilterGrayscale:
		convertOptions.Colored = false 
		convertOptions.Reversed = false
        
	case FilterDuotone:
		convertOptions.Colored = false 
		convertOptions.Reversed = true
        
	case FilterInverted:
		convertOptions.Colored = true 
		convertOptions.Reversed = true 
	}

	return converter.ImageFile2ASCIIString(path, &convertOptions), nil
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
