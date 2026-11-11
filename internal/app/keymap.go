package app

type FocusArea int

const (
	FocusSidebar FocusArea = iota
	FocusEditor
	FocusMenu
)

type SidebarView int

const (
	SidebarFiles SidebarView = iota
	SidebarGit
)

type KeyAction int

const (
	ActionNone KeyAction = iota
	ActionQuit
	ActionToggleSidebar
	ActionSwitchFocus
	ActionToggleAutoSave
	ActionSidebarFiles
	ActionSidebarGit
	ActionHelp
)
