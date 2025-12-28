// Package tui provides the Bubble Tea TUI for mono-commander.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme colors - semantic color palette
var (
	// Base colors
	ColorBg        = lipgloss.Color("235") // Dark background
	ColorBgCard    = lipgloss.Color("236") // Slightly lighter for cards
	ColorBorder    = lipgloss.Color("238") // Subtle border
	ColorBorderFoc = lipgloss.Color("99")  // Focused border (accent)

	// Text colors
	ColorMuted  = lipgloss.Color("241") // Labels, static text
	ColorNormal = lipgloss.Color("252") // Paragraphs
	ColorBright = lipgloss.Color("255") // Dynamic values, emphasis
	ColorAccent = lipgloss.Color("99")  // Action keys, accent (purple)

	// Semantic colors
	ColorSuccess = lipgloss.Color("82")  // Green
	ColorWarning = lipgloss.Color("220") // Yellow
	ColorDanger  = lipgloss.Color("196") // Red
	ColorInfo    = lipgloss.Color("75")  // Cyan/blue
)

// Text styles
var (
	// TextMuted for labels, static text
	TextMuted = lipgloss.NewStyle().Foreground(ColorMuted)

	// TextNormal for paragraphs
	TextNormal = lipgloss.NewStyle().Foreground(ColorNormal)

	// TextBright for dynamic values
	TextBright = lipgloss.NewStyle().Foreground(ColorBright)

	// TextAction for keys/buttons
	TextAction = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	// TextSuccess for success messages
	TextSuccess = lipgloss.NewStyle().Foreground(ColorSuccess)

	// TextWarning for warnings
	TextWarning = lipgloss.NewStyle().Foreground(ColorWarning)

	// TextDanger for errors
	TextDanger = lipgloss.NewStyle().Foreground(ColorDanger)

	// TextInfo for informational messages
	TextInfo = lipgloss.NewStyle().Foreground(ColorInfo)
)

// Tab bar styles
var (
	TabInactive = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(ColorMuted)

	TabActive = lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Foreground(ColorBright).
			Background(lipgloss.Color("237"))

	TabBarDivider = lipgloss.NewStyle().
			Foreground(ColorBorder).
			SetString(strings.Repeat("─", 100))
)

// Card styles
var (
	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	CardTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent).
			MarginBottom(1)

	CardFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorderFoc).
				Padding(1, 2)
)

// Header styles
var (
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	SubHeaderStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)

	PageTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBright).
			MarginBottom(1)
)

// Badge styles for status indicators
type BadgeType int

const (
	BadgeOK BadgeType = iota
	BadgeWarn
	BadgeFail
	BadgeNA
	BadgeInfo
)

// Badge creates a styled status badge
func Badge(t BadgeType, text string) string {
	var style lipgloss.Style

	switch t {
	case BadgeOK:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("235")).
			Background(ColorSuccess).
			Padding(0, 1).
			Bold(true)
		if text == "" {
			text = "OK"
		}
	case BadgeWarn:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("235")).
			Background(ColorWarning).
			Padding(0, 1).
			Bold(true)
		if text == "" {
			text = "WARN"
		}
	case BadgeFail:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			Background(ColorDanger).
			Padding(0, 1).
			Bold(true)
		if text == "" {
			text = "FAIL"
		}
	case BadgeNA:
		style = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Background(lipgloss.Color("238")).
			Padding(0, 1)
		if text == "" {
			text = "N/A"
		}
	case BadgeInfo:
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("235")).
			Background(ColorInfo).
			Padding(0, 1).
			Bold(true)
		if text == "" {
			text = "INFO"
		}
	}

	return style.Render(text)
}

// StatusBadge returns an appropriate badge for a status string
func StatusBadge(status string) string {
	switch strings.ToLower(status) {
	case "ok", "pass", "active", "running", "synced", "bonded":
		return Badge(BadgeOK, strings.ToUpper(status))
	case "warn", "warning", "catching up", "unbonding":
		return Badge(BadgeWarn, strings.ToUpper(status))
	case "fail", "failed", "error", "jailed", "inactive":
		return Badge(BadgeFail, strings.ToUpper(status))
	case "n/a", "unknown", "not configured", "not installed":
		return Badge(BadgeNA, "N/A")
	default:
		return Badge(BadgeInfo, status)
	}
}

// BoolBadge returns OK/FAIL badge based on boolean
func BoolBadge(ok bool) string {
	if ok {
		return Badge(BadgeOK, "OK")
	}
	return Badge(BadgeFail, "FAIL")
}

// Card creates a styled card with title and body
func Card(title, body string, width int) string {
	if width == 0 {
		width = 40
	}

	titleRendered := CardTitleStyle.Render(title)
	content := titleRendered + "\n" + body

	return CardStyle.Width(width).Render(content)
}

// CardFocused creates a focused card (highlighted border)
func CardFocused(title, body string, width int) string {
	if width == 0 {
		width = 40
	}

	titleRendered := CardTitleStyle.Render(title)
	content := titleRendered + "\n" + body

	return CardFocusedStyle.Width(width).Render(content)
}

// KeyHint creates a keyboard hint like "[k] action"
func KeyHint(key, action string) string {
	keyStyle := lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true)

	actionStyle := lipgloss.NewStyle().
		Foreground(ColorMuted)

	return keyStyle.Render("["+key+"]") + " " + actionStyle.Render(action)
}

// KeyHints joins multiple key hints
func KeyHints(hints ...string) string {
	return strings.Join(hints, "  ")
}

// Table renders a simple two-column table
func Table(rows [][]string, indent int) string {
	if len(rows) == 0 {
		return ""
	}

	// Find max width for first column
	maxWidth := 0
	for _, row := range rows {
		if len(row) > 0 && lipgloss.Width(row[0]) > maxWidth {
			maxWidth = lipgloss.Width(row[0])
		}
	}

	var b strings.Builder
	indentStr := strings.Repeat(" ", indent)
	labelStyle := TextMuted
	valueStyle := TextBright

	for _, row := range rows {
		b.WriteString(indentStr)
		if len(row) >= 2 {
			label := labelStyle.Render(row[0])
			// Pad to align values
			padding := maxWidth - lipgloss.Width(row[0])
			b.WriteString(label)
			b.WriteString(strings.Repeat(" ", padding+2))
			b.WriteString(valueStyle.Render(row[1]))
		} else if len(row) == 1 {
			b.WriteString(row[0])
		}
		b.WriteString("\n")
	}
	return b.String()
}

// StatusTable renders a table with status badges
type StatusRow struct {
	Label  string
	Status BadgeType
	Value  string
	Note   string
}

func StatusTable(rows []StatusRow, indent int) string {
	var b strings.Builder
	indentStr := strings.Repeat(" ", indent)

	// Find max width for label column
	maxLabelWidth := 0
	for _, row := range rows {
		if lipgloss.Width(row.Label) > maxLabelWidth {
			maxLabelWidth = lipgloss.Width(row.Label)
		}
	}

	for _, row := range rows {
		b.WriteString(indentStr)
		label := TextMuted.Render(row.Label)
		padding := maxLabelWidth - lipgloss.Width(row.Label)
		b.WriteString(label)
		b.WriteString(strings.Repeat(" ", padding+2))
		b.WriteString(Badge(row.Status, row.Value))
		if row.Note != "" {
			b.WriteString("  ")
			b.WriteString(TextMuted.Render(row.Note))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// WarningBox creates a warning box with message
func WarningBox(title, message string, width int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorWarning).
		Foreground(ColorWarning).
		Padding(1, 2).
		Width(width)

	content := TextWarning.Bold(true).Render("⚠ "+title) + "\n" + message
	return style.Render(content)
}

// ErrorBox creates an error box with message
func ErrorBox(title, message string, width int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorDanger).
		Foreground(ColorDanger).
		Padding(1, 2).
		Width(width)

	content := TextDanger.Bold(true).Render("✗ "+title) + "\n" + message
	return style.Render(content)
}

// InfoBox creates an info box with message
func InfoBox(title, message string, width int) string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorInfo).
		Padding(1, 2).
		Width(width)

	content := TextInfo.Bold(true).Render("ℹ "+title) + "\n" + TextNormal.Render(message)
	return style.Render(content)
}

// Divider creates a horizontal divider
func Divider(width int) string {
	return lipgloss.NewStyle().
		Foreground(ColorBorder).
		Render(strings.Repeat("─", width))
}

// ProgressIndicator shows a simple progress indicator
func ProgressIndicator(label string, current, total int) string {
	pct := 0
	if total > 0 {
		pct = (current * 100) / total
	}

	barWidth := 20
	filled := (pct * barWidth) / 100
	empty := barWidth - filled

	bar := TextSuccess.Render(strings.Repeat("█", filled)) +
		TextMuted.Render(strings.Repeat("░", empty))

	return fmt.Sprintf("%s %s %d%%", TextMuted.Render(label), bar, pct)
}

// ScrollHint creates a scroll hint for viewports
func ScrollHint(atTop, atBottom bool) string {
	hint := TextMuted.Render("Scroll: ")
	if atTop {
		hint += TextMuted.Render("↓ PgDn End")
	} else if atBottom {
		hint += TextMuted.Render("↑ PgUp Home")
	} else {
		hint += TextMuted.Render("↑↓ PgUp PgDn")
	}
	return hint
}

// TabPositions stores the X positions of each tab for mouse click detection
type TabPositions struct {
	Tabs []TabPosition
}

type TabPosition struct {
	Tab   Tab
	Start int
	End   int
}

// RenderTabBar renders the tab bar and returns positions for mouse detection
func RenderTabBar(tabs []Tab, activeTab Tab, width int) (string, TabPositions) {
	var renderedTabs []string
	var positions TabPositions
	currentX := 0

	for _, t := range tabs {
		var style lipgloss.Style
		if t == activeTab {
			style = TabActive
		} else {
			style = TabInactive
		}
		rendered := style.Render(t.String())
		renderedWidth := lipgloss.Width(rendered)

		positions.Tabs = append(positions.Tabs, TabPosition{
			Tab:   t,
			Start: currentX,
			End:   currentX + renderedWidth,
		})

		currentX += renderedWidth + 1 // +1 for space
		renderedTabs = append(renderedTabs, rendered)
	}

	tabBar := strings.Join(renderedTabs, " ")
	divider := Divider(width)

	return tabBar + "\n" + divider, positions
}

// GetTabAtPosition returns the tab at the given X position, or -1 if none
func (tp TabPositions) GetTabAtPosition(x int) Tab {
	for _, pos := range tp.Tabs {
		if x >= pos.Start && x < pos.End {
			return pos.Tab
		}
	}
	return Tab(-1)
}

// ActionBar renders a bottom action bar with key hints
func ActionBar(hints []string, width int) string {
	style := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(width).
		Align(lipgloss.Center)

	return style.Render(strings.Join(hints, "  •  "))
}

// PageHeader renders a page header with title and optional subtitle
func PageHeader(title, subtitle string) string {
	header := PageTitleStyle.Render(title)
	if subtitle != "" {
		header += "  " + SubHeaderStyle.Render(subtitle)
	}
	return header
}

// WizardStep renders a wizard step indicator
type WizardStep struct {
	Number   int
	Title    string
	Status   BadgeType
	Active   bool
	Complete bool
}

func RenderWizardSteps(steps []WizardStep, width int) string {
	var b strings.Builder

	for _, step := range steps {
		prefix := fmt.Sprintf("%d. ", step.Number)
		var stepStyle lipgloss.Style

		if step.Active {
			stepStyle = TextBright.Copy().Bold(true)
			prefix = "▶ " + prefix
		} else if step.Complete {
			stepStyle = TextSuccess
			prefix = "✓ " + prefix
		} else {
			stepStyle = TextMuted
			prefix = "  " + prefix
		}

		b.WriteString(stepStyle.Render(prefix + step.Title))

		if step.Status != BadgeNA || step.Active {
			b.WriteString("  ")
			if step.Complete {
				b.WriteString(Badge(BadgeOK, "Done"))
			} else if step.Active {
				b.WriteString(Badge(BadgeInfo, "Current"))
			} else {
				b.WriteString(Badge(step.Status, ""))
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// NetworkMismatchWarning renders a network mismatch warning
func NetworkMismatchWarning(selected, actual string, width int) string {
	title := "Network Mismatch"
	message := fmt.Sprintf(
		"Selected: %s\nNode reports: %s\n\nPress [n] to change network or rejoin.",
		TextBright.Render(selected),
		TextDanger.Render(actual),
	)
	return WarningBox(title, message, width)
}
