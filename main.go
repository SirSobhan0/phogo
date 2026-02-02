package main

import (
	"flag"
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/qeesung/image2ascii/convert"
)

// --- Types & Constants ---

type sessionState int

const (
	stateBrowsing sessionState = iota
	stateViewingImage
	stateDirBrowsing
	stateSearching
	stateConfirmDelete
	stateRenaming
)

const (
	SortName = iota
	SortSize
	SortDate
)

const (
	FilterColor     = "Color"
	FilterGrayscale = "Grayscale"
	FilterInverted  = "Inverted"
	FilterDuotone   = "Duotone"
)

type imageRenderedMsg string
type tickMsg time.Time

// --- Styles ---

var (
	appStyle   = lipgloss.NewStyle().Padding(1, 2)
	titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#25A065")).Padding(0, 1)
	alertStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#FF5555")).Padding(0, 1).Bold(true)
	infoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#61AFEF")).Padding(0, 1).Bold(true)

	helpKeyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#25A065")).Bold(true)
	helpDescStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#777777"))
	dirStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#99CCFF"))
)

type item struct {
	title, fileName string
	isDir           bool
	size            int64
	modTime         int64
}

func (i item) Title() string {
	if i.isDir {
		return dirStyle.Render("📁 " + i.title)
	}
	return i.title
}
func (i item) Description() string {
	if i.isDir {
		return "Directory"
	}
	return formatBytes(i.size)
}
func (i item) FilterValue() string { return i.title }

type model struct {
	state            sessionState
	list             list.Model
	viewport         viewport.Model
	searchInput    textinput.Model
	renameInput    textinput.Model
	currentDir       string
	dirBrowserPath   string
	searchQuery    string
	showHidden       bool
	statusMsg        string
	imgContent       string
	isRendering    bool
	isSlideshow    bool
	prevDirState     sessionState
	filterMode       string
	sortMode         int
	initialImagePath string
}

// --- Helpers ---

func copyToClipboard(content string) {
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(content)
	_ = cmd.Run()
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func tick() tea.Cmd {
	return tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func renderImageCmd(path string, w, h int, mode string) tea.Cmd {
	return func() tea.Msg {
		return imageRenderedMsg(renderImage(path, w, h, mode))
	}
}

func initialModel(startPath string) model {
	absPath, _ := filepath.Abs(startPath)
	fileInfo, err := os.Stat(absPath)
	targetDir := absPath
	targetImg := ""

	if err == nil && !fileInfo.IsDir() {
		targetDir = filepath.Dir(absPath)
		targetImg = filepath.Base(absPath)
	}

	l := list.New(getFiles(targetDir, false, "", false, SortName), list.NewDefaultDelegate(), 0, 0)
	l.SetShowHelp(false)

	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.NoItems = lipgloss.NewStyle()
	
	l.KeyMap.NextPage = key.NewBinding(key.WithKeys("pgdown", "right"))
	l.KeyMap.PrevPage = key.NewBinding(key.WithKeys("pgup", "left"))
	l.KeyMap.CursorUp.SetKeys("up", "k")
	l.KeyMap.CursorDown.SetKeys("down", "j")

	return model{
		state:            stateBrowsing,
		list:             l,
		viewport:         viewport.New(0, 0),
		searchInput:      textinput.New(),
		renameInput:      textinput.New(),
		currentDir:       targetDir,
		dirBrowserPath:   targetDir,
		filterMode:       FilterColor,
		sortMode:         SortName,
		initialImagePath: targetImg,
	}
}

func (m model) Init() tea.Cmd {
	if m.initialImagePath != "" {
		for i, itm := range m.list.Items() {
			if itm.(item).fileName == m.initialImagePath {
				m.list.Select(i)
				break
			}
		}
		path := filepath.Join(m.currentDir, m.initialImagePath)
		return renderImageCmd(path, 120, 40, m.filterMode)
	}
	return nil
}

func (m *model) reloadImageList() {
	m.list.SetItems(getFiles(m.currentDir, false, m.searchQuery, m.showHidden, m.sortMode))
}

func (m *model) reloadDirList() {
	m.list.SetItems(getFiles(m.dirBrowserPath, true, m.searchQuery, m.showHidden, m.sortMode))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width-4, msg.Height-10)
		m.viewport.Width, m.viewport.Height = msg.Width, msg.Height-10
		if m.initialImagePath != "" {
			m.state, m.isRendering = stateViewingImage, true
			m.initialImagePath = ""
			return m, renderImageCmd(filepath.Join(m.currentDir, m.list.SelectedItem().(item).fileName), m.viewport.Width, m.viewport.Height, m.filterMode)
		}

	case imageRenderedMsg:
		m.isRendering = false
		m.imgContent = string(msg)
		m.viewport.SetContent(m.imgContent)

	case tickMsg:
		if m.isSlideshow && m.state == stateViewingImage {
			if m.list.Index() == len(m.list.Items())-1 {
				m.list.Select(0)
			} else {
				m.list.CursorDown()
			}
			if itm, ok := m.list.SelectedItem().(item); ok {
				path := filepath.Join(m.currentDir, itm.fileName)
				return m, tea.Batch(tick(), renderImageCmd(path, m.viewport.Width, m.viewport.Height, m.filterMode))
			}
		}
		return m, nil

	case tea.KeyMsg:
		keyStr := msg.String()
		if keyStr == "ctrl+c" { return m, tea.Quit }
		if m.isSlideshow { m.isSlideshow = false; m.statusMsg = "Slideshow Stopped"; return m, nil }

		switch m.state {
		case stateBrowsing:
			switch keyStr {
			case "q": return m, tea.Quit
			case "enter":
				if itm, ok := m.list.SelectedItem().(item); ok && !itm.isDir {
					path := filepath.Join(m.currentDir, itm.fileName)
					m.state, m.isRendering = stateViewingImage, true
					return m, renderImageCmd(path, m.viewport.Width, m.viewport.Height, m.filterMode)
				}
			case "P":
				if itm, ok := m.list.SelectedItem().(item); ok && !itm.isDir {
					m.isSlideshow, m.state, m.isRendering = true, stateViewingImage, true
					return m, tea.Batch(tick(), renderImageCmd(filepath.Join(m.currentDir, itm.fileName), m.viewport.Width, m.viewport.Height, m.filterMode))
				}
			case "d": m.state = stateDirBrowsing; m.reloadDirList()
			case "x": m.state = stateConfirmDelete
			case "r":
				if itm, ok := m.list.SelectedItem().(item); ok {
					m.state = stateRenaming
					m.renameInput.SetValue(itm.fileName)
					m.renameInput.Focus()
				}
			case "s": m.sortMode = (m.sortMode + 1) % 3; m.reloadImageList()
			case "y":
				if itm, ok := m.list.SelectedItem().(item); ok {
					path, _ := filepath.Abs(filepath.Join(m.currentDir, itm.fileName))
					copyToClipboard(path); m.statusMsg = "Path Copied!"
				}
			case "h": m.showHidden = !m.showHidden; m.reloadImageList()
			case "/": m.state, m.prevDirState = stateSearching, stateBrowsing; m.searchInput.Focus()
			}
			m.list, cmd = m.list.Update(msg); cmds = append(cmds, cmd)

		case stateViewingImage:
			switch keyStr {
			case "esc", "q": m.state, m.statusMsg = stateBrowsing, ""
			case "1", "2", "3", "4":
				modes := []string{FilterColor, FilterGrayscale, FilterInverted, FilterDuotone}
				idx, _ := strconv.Atoi(keyStr)
				m.filterMode, m.isRendering = modes[idx-1], true
				path := filepath.Join(m.currentDir, m.list.SelectedItem().(item).fileName)
				return m, renderImageCmd(path, m.viewport.Width, m.viewport.Height, m.filterMode)
			}
			m.viewport, cmd = m.viewport.Update(msg); cmds = append(cmds, cmd)

		case stateDirBrowsing:
			if keyStr == "enter" {
				if itm, ok := m.list.SelectedItem().(item); ok {
					newP := filepath.Join(m.dirBrowserPath, itm.fileName)
					if itm.fileName == ".." { newP = filepath.Dir(m.dirBrowserPath) }
					if info, err := os.Stat(newP); err == nil && info.IsDir() {
						m.dirBrowserPath = newP; m.reloadDirList()
					}
				}
			} else if keyStr == "d" { m.currentDir, m.state = m.dirBrowserPath, stateBrowsing; m.reloadImageList()
			} else if keyStr == "h" { m.showHidden = !m.showHidden; m.reloadDirList()
			} else if keyStr == "esc" { m.state = stateBrowsing }
			m.list, cmd = m.list.Update(msg); cmds = append(cmds, cmd)

		case stateSearching:
			if keyStr == "enter" {
				m.searchQuery, m.state = m.searchInput.Value(), m.prevDirState
				if m.state == stateDirBrowsing { m.reloadDirList() } else { m.reloadImageList() }
			} else if keyStr == "esc" { m.state = m.prevDirState }
			m.searchInput, cmd = m.searchInput.Update(msg); cmds = append(cmds, cmd)

		case stateRenaming:
			if keyStr == "enter" {
				os.Rename(filepath.Join(m.currentDir, m.list.SelectedItem().(item).fileName), filepath.Join(m.currentDir, m.renameInput.Value()))
				m.state = stateBrowsing; m.reloadImageList()
			} else if keyStr == "esc" { m.state = stateBrowsing }
			m.renameInput, cmd = m.renameInput.Update(msg); cmds = append(cmds, cmd)

		case stateConfirmDelete:
			if keyStr == "y" { os.Remove(filepath.Join(m.currentDir, m.list.SelectedItem().(item).fileName)); m.reloadImageList() }
			m.state = stateBrowsing
		}
	}
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	sortNames := []string{"Name", "Size", "Date"}
	status := fmt.Sprintf("Sort: %s | Hidden: %v", sortNames[m.sortMode], m.showHidden)
	if m.isSlideshow { status = "SLIDESHOW LOOPING (Any key to stop)" }

	switch m.state {
	case stateBrowsing:
		return appStyle.Render(fmt.Sprintf("%s | %s\n%s\n\n%s\n%s", titleStyle.Render("phogo"), status, m.currentDir, m.list.View(), m.renderHelp([]string{"j/k", "move", "P", "slide", "s", "sort", "y", "path", "r", "name", "x", "del", "h", "hide", "/", "find"})))
	case stateViewingImage:
		content := m.viewport.View()
		if m.isRendering { content = "\n\n  Rendering..." }
		return fmt.Sprintf("%s\n%s\n%s", titleStyle.Render("Image View"), content, m.renderHelp([]string{"1-4", "filter", "esc", "back"}))
	case stateDirBrowsing:
		return appStyle.Render(fmt.Sprintf("%s\n%s\n\n%s\n%s", titleStyle.Render("FOLDERS"), m.dirBrowserPath, m.list.View(), m.renderHelp([]string{"j/k", "move", "enter", "open", "d", "set", "h", "hide", "esc", "back"})))
	case stateSearching, stateRenaming, stateConfirmDelete:
		prompt := "SEARCH"; input := m.searchInput.View()
		if m.state == stateRenaming { prompt, input = "RENAME", m.renameInput.View() }
		if m.state == stateConfirmDelete { prompt, input = "DELETE?", m.renderHelp([]string{"y", "yes", "n", "no"}) }
		return appStyle.Render(infoStyle.Render(prompt) + "\n" + input)
	}
	return ""
}

func (m model) renderHelp(keys []string) string {
	var h []string
	for i := 0; i < len(keys); i += 2 { h = append(h, fmt.Sprintf("%s %s", helpKeyStyle.Render(keys[i]), helpDescStyle.Render(keys[i+1]))) }
	return "\n" + strings.Join(h, " • ") + "\n" + m.statusMsg
}

func getFiles(dir string, dirsOnly bool, searchQuery string, showHidden bool, sortMode int) []list.Item {
	entries, _ := os.ReadDir(dir)
	var items []item
	query := strings.ToLower(searchQuery)
	if dirsOnly { items = append(items, item{title: "..", fileName: "..", isDir: true}) }
	for _, e := range entries {
		name := e.Name()
		if !showHidden && strings.HasPrefix(name, ".") { continue }
		if searchQuery != "" && !strings.Contains(strings.ToLower(name), query) { continue }
		info, _ := e.Info()
		itm := item{title: name, fileName: name, isDir: e.IsDir(), size: info.Size(), modTime: info.ModTime().Unix()}
		if e.IsDir() {
			if dirsOnly { items = append(items, itm) }
		} else if !dirsOnly {
			ext := strings.ToLower(filepath.Ext(name))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" { items = append(items, itm) }
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].title == ".." { return true }; if items[j].title == ".." { return false }
		switch sortMode {
		case SortSize: return items[i].size > items[j].size
		case SortDate: return items[i].modTime > items[j].modTime
		default: return strings.ToLower(items[i].title) < strings.ToLower(items[j].title)
		}
	})
	var res []list.Item
	for _, itm := range items { res = append(res, itm) }
	return res
}

func renderImage(path string, w, h int, filterMode string) string {
	opts := convert.DefaultOptions
	opts.FixedWidth, opts.FixedHeight = w, h
	switch filterMode {
	case FilterColor: opts.Colored, opts.Reversed = true, false
	case FilterGrayscale: opts.Colored, opts.Reversed = false, false
	case FilterDuotone: opts.Colored, opts.Reversed = false, true
	case FilterInverted: opts.Colored, opts.Reversed = true, true
	}
	return convert.NewImageConverter().ImageFile2ASCIIString(path, &opts)
}

func main() {
	isConvert := flag.Bool("convert", false, "Spit out ASCII to stdout and exit")
	flag.Parse()
	args := flag.Args()

	startPath := "."
	if len(args) > 0 { startPath = args[0] }

	if *isConvert {
		fmt.Print(renderImage(startPath, 80, 40, FilterColor))
		os.Exit(0)
	}

	p := tea.NewProgram(initialModel(startPath), tea.WithAltScreen())
	if _, err := p.Run(); err != nil { os.Exit(1) }
}
