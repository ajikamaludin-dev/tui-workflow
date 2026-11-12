package ui

import "github.com/charmbracelet/lipgloss"

type Theme struct {
	BgPrimary     lipgloss.Color
	BgSidebar     lipgloss.Color
	BgTopMenu     lipgloss.Color
	BgStatusBar   lipgloss.Color
	BgActiveTab   lipgloss.Color
	BgInactiveTab lipgloss.Color
	BorderColor   lipgloss.Color
	FgPrimary     lipgloss.Color
	FgSecondary   lipgloss.Color
	FgHighlight   lipgloss.Color
	Accent        lipgloss.Color
	Red           lipgloss.Color
	Green         lipgloss.Color
	Yellow        lipgloss.Color

	// Pre-built styles
	TopMenuStyle      lipgloss.Style
	TopMenuItemStyle  lipgloss.Style
	TopMenuActiveItem lipgloss.Style
	DropdownStyle     lipgloss.Style
	DropdownItemStyle lipgloss.Style

	SidebarStyle        lipgloss.Style
	SidebarHeaderStyle  lipgloss.Style
	SidebarTabActive    lipgloss.Style
	SidebarTabInactive  lipgloss.Style

	EditorStyle         lipgloss.Style
	TabBarContainer     lipgloss.Style
	TabActiveStyle      lipgloss.Style
	TabInactiveStyle    lipgloss.Style

	StatusBarStyle      lipgloss.Style
	StatusBarItemStyle  lipgloss.Style
	StatusBarBadgeStyle lipgloss.Style

	WarningBoxStyle     lipgloss.Style
	LineNumberStyle     lipgloss.Style

	SearchMatchStyle    lipgloss.Style
	SearchActiveMatch   lipgloss.Style
}

var CurrentTheme = NewDarkTheme()

func NewDarkTheme() Theme {
	t := Theme{
		BgPrimary:     lipgloss.Color("#1E1E1E"),
		BgSidebar:     lipgloss.Color("#252526"),
		BgTopMenu:     lipgloss.Color("#2D2D2D"),
		BgStatusBar:   lipgloss.Color("#007ACC"),
		BgActiveTab:   lipgloss.Color("#1E1E1E"),
		BgInactiveTab: lipgloss.Color("#2D2D2D"),
		BorderColor:   lipgloss.Color("#3C3C3C"),
		FgPrimary:     lipgloss.Color("#CCCCCC"),
		FgSecondary:   lipgloss.Color("#858585"),
		FgHighlight:   lipgloss.Color("#FFFFFF"),
		Accent:        lipgloss.Color("#0E639C"),
		Red:           lipgloss.Color("#F14C4C"),
		Green:         lipgloss.Color("#89D185"),
		Yellow:        lipgloss.Color("#CCA700"),
	}

	t.TopMenuStyle = lipgloss.NewStyle().
		Background(t.BgTopMenu).
		Foreground(t.FgPrimary)

	t.TopMenuItemStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Foreground(t.FgPrimary)

	t.TopMenuActiveItem = lipgloss.NewStyle().
		Padding(0, 1).
		Background(t.Accent).
		Foreground(t.FgHighlight).
		Bold(true)

	t.DropdownStyle = lipgloss.NewStyle().
		Background(t.BgTopMenu).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderColor)

	t.DropdownItemStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Foreground(t.FgPrimary)

	t.SidebarStyle = lipgloss.NewStyle().
		Background(t.BgSidebar).
		Foreground(t.FgPrimary).
		BorderRight(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(t.BorderColor)

	t.SidebarHeaderStyle = lipgloss.NewStyle().
		Background(t.BgTopMenu).
		Foreground(t.FgSecondary)

	t.SidebarTabActive = lipgloss.NewStyle().
		Padding(0, 1).
		Background(t.BgSidebar).
		Foreground(t.FgHighlight).
		Bold(true)

	t.SidebarTabInactive = lipgloss.NewStyle().
		Padding(0, 1).
		Background(t.BgTopMenu).
		Foreground(t.FgSecondary)

	t.EditorStyle = lipgloss.NewStyle().
		Background(t.BgPrimary).
		Foreground(t.FgPrimary)

	t.TabBarContainer = lipgloss.NewStyle().
		Background(t.BgInactiveTab)

	t.TabActiveStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Background(t.BgActiveTab).
		Foreground(t.FgHighlight).
		Bold(true)

	t.TabInactiveStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Background(t.BgInactiveTab).
		Foreground(t.FgSecondary)

	t.StatusBarStyle = lipgloss.NewStyle().
		Background(t.BgStatusBar).
		Foreground(t.FgHighlight)

	t.StatusBarItemStyle = lipgloss.NewStyle().
		Padding(0, 1)

	t.StatusBarBadgeStyle = lipgloss.NewStyle().
		Padding(0, 1).
		Background(lipgloss.Color("#16825D")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	t.WarningBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(t.Red).
		Padding(1, 2).
		Foreground(t.FgHighlight).
		Bold(true)

	t.LineNumberStyle = lipgloss.NewStyle().
		Foreground(t.FgSecondary)

	t.SearchMatchStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#515C6B")).
		Foreground(lipgloss.Color("#FFFFFF"))

	t.SearchActiveMatch = lipgloss.NewStyle().
		Background(lipgloss.Color("#E5A50A")).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	return t
}
