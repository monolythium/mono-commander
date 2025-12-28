// Package tui provides the Bubble Tea TUI for mono-commander.
package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Terminal capability detection
var (
	// HasTrueColor indicates if terminal supports true color (24-bit)
	HasTrueColor = detectTrueColor()
)

// detectTrueColor checks if terminal supports true color
func detectTrueColor() bool {
	colorTerm := os.Getenv("COLORTERM")
	return colorTerm == "truecolor" || colorTerm == "24bit"
}

// SupportsTrueColor returns whether the terminal supports 24-bit color
func SupportsTrueColor() bool {
	return HasTrueColor
}

// Theme colors - premium Monolythium palette
var (
	// Brand colors (gradient-like for monolith feel)
	ColorBrand       = lipgloss.Color("99")  // Primary purple (Monolythium accent)
	ColorBrandBright = lipgloss.Color("141") // Lighter purple
	ColorBrandDim    = lipgloss.Color("61")  // Darker purple

	// App-wide background (truecolor: #0B0F14, fallback: 235)
	ColorAppBg       = getAppBackground()
	ColorAppBgCard   = getCardBackground()
	ColorAppBgHeader = getHeaderBackground()

	// Base colors (legacy, kept for compatibility)
	ColorBg        = lipgloss.Color("235") // Dark background
	ColorBgCard    = lipgloss.Color("236") // Slightly lighter for cards
	ColorBgHeader  = lipgloss.Color("234") // Darker for header
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

// getAppBackground returns the app background color based on terminal capability
func getAppBackground() lipgloss.Color {
	if HasTrueColor {
		return lipgloss.Color("#0B0F14") // Modern dark background
	}
	return lipgloss.Color("235") // Fallback ANSI dark
}

// getCardBackground returns the card background color
func getCardBackground() lipgloss.Color {
	if HasTrueColor {
		return lipgloss.Color("#12171D") // Slightly lighter for cards
	}
	return lipgloss.Color("236")
}

// getHeaderBackground returns the header background color
func getHeaderBackground() lipgloss.Color {
	if HasTrueColor {
		return lipgloss.Color("#080A0D") // Darker for header
	}
	return lipgloss.Color("234")
}

// Gradient border colors for truecolor terminals
var gradientColors = []string{
	"#6A35FF", // Start - vibrant purple
	"#7445FF",
	"#7E55FF",
	"#8866FF",
	"#9277FF",
	"#9C88FF",
	"#A699FF",
	"#B08CFF", // End - lighter purple
}

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

// Tab bar styles - premium outlined design
var (
	// Inactive tab: simple text
	TabInactive = lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(ColorMuted)

	// Active tab: outlined box with accent border
	TabActive = lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Foreground(ColorBright).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBrand).
			BorderTop(true).
			BorderBottom(true).
			BorderLeft(true).
			BorderRight(true)

	// Tab bar container
	TabBarContainer = lipgloss.NewStyle().
				PaddingLeft(1).
				MarginTop(0).
				MarginBottom(0)

	// Tab bar bottom border
	TabBarBorder = lipgloss.NewStyle().
			Foreground(ColorBorder)
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

// Branding header bar styles
var (
	// Monolith icon (ASCII art representation)
	MonolithIcon = "◆"

	// Brand title style
	BrandTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBrand).
			PaddingRight(1)

	// Commander text style
	CommanderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBright)

	// Network badge style
	NetworkBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("235")).
				Background(ColorBrand).
				Padding(0, 1).
				Bold(true)

	// Version badge style
	VersionBadgeStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Background(lipgloss.Color("238")).
				Padding(0, 1)

	// Header bar container
	HeaderBarStyle = lipgloss.NewStyle().
			Background(ColorBgHeader).
			Padding(0, 1).
			MarginBottom(0)

	// Header info style (chain-id, height, etc.)
	HeaderInfoStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Header value style
	HeaderValueStyle = lipgloss.NewStyle().
				Foreground(ColorBright)
)

// RenderBrandingHeader renders the top branding header bar
func RenderBrandingHeader(network, chainID string, height int64, version string, updateOK bool, width int) string {
	// Left side: MONO COMMANDER with icon
	icon := BrandTitleStyle.Render(MonolithIcon)
	title := CommanderStyle.Render("MONO COMMANDER")
	leftContent := icon + " " + title

	// Center: Network badge + chain info
	networkBadge := NetworkBadgeStyle.Render(strings.ToUpper(network))

	chainInfo := ""
	if chainID != "" {
		chainInfo += HeaderInfoStyle.Render(" · ") + HeaderValueStyle.Render(chainID)
	}
	if height > 0 {
		chainInfo += HeaderInfoStyle.Render(" · H:") + HeaderValueStyle.Render(fmt.Sprintf("%d", height))
	}

	centerContent := networkBadge + chainInfo

	// Right side: Version badge + update status
	versionBadge := VersionBadgeStyle.Render("v" + version)
	var statusBadge string
	if updateOK {
		statusBadge = " " + Badge(BadgeOK, "")
	} else {
		statusBadge = " " + Badge(BadgeWarn, "UPDATE")
	}
	rightContent := versionBadge + statusBadge

	// Calculate spacing
	leftWidth := lipgloss.Width(leftContent)
	centerWidth := lipgloss.Width(centerContent)
	rightWidth := lipgloss.Width(rightContent)

	// Calculate gaps
	totalContentWidth := leftWidth + centerWidth + rightWidth
	remainingSpace := width - totalContentWidth - 4 // 4 for padding

	if remainingSpace < 0 {
		remainingSpace = 0
	}

	leftGap := remainingSpace / 2
	rightGap := remainingSpace - leftGap

	// Build header line
	header := leftContent +
		strings.Repeat(" ", leftGap) +
		centerContent +
		strings.Repeat(" ", rightGap) +
		rightContent

	// Apply header bar style
	return HeaderBarStyle.Width(width).Render(header)
}

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

// GradientCard creates a card with pseudo-gradient border (truecolor only)
// Falls back to standard card if truecolor is not supported
func GradientCard(title, body string, width int) string {
	if width == 0 {
		width = 40
	}

	// Fallback to standard card if no truecolor
	if !HasTrueColor {
		return Card(title, body, width)
	}

	titleRendered := CardTitleStyle.Render(title)
	content := titleRendered + "\n" + body

	// Split content into lines and calculate dimensions
	contentLines := strings.Split(content, "\n")
	innerWidth := width - 4 // Account for border and padding

	// Pad/truncate content lines to inner width
	paddedLines := make([]string, len(contentLines))
	for i, line := range contentLines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < innerWidth {
			paddedLines[i] = line + strings.Repeat(" ", innerWidth-lineWidth)
		} else {
			paddedLines[i] = line[:innerWidth]
		}
	}

	return renderGradientBorder(paddedLines, width)
}

// renderGradientBorder renders content with a gradient border
func renderGradientBorder(contentLines []string, width int) string {
	var b strings.Builder

	innerWidth := width - 4 // 2 for borders, 2 for padding
	if innerWidth < 1 {
		innerWidth = 1
	}

	// Calculate gradient step
	numColors := len(gradientColors)

	// Top border: ╭ ─ ─ ─ ╮ with gradient
	topBorder := renderGradientTopBorder(innerWidth+2, numColors)
	b.WriteString(topBorder)
	b.WriteString("\n")

	// Side border color (use mid gradient color)
	sideColor := lipgloss.Color(gradientColors[numColors/2])
	sideStyle := lipgloss.NewStyle().Foreground(sideColor)

	// Content lines with side borders
	bgStyle := lipgloss.NewStyle().Background(ColorAppBgCard)
	for _, line := range contentLines {
		b.WriteString(sideStyle.Render("│"))
		b.WriteString(bgStyle.Render(" " + line + " "))
		b.WriteString(sideStyle.Render("│"))
		b.WriteString("\n")
	}

	// Bottom border: ╰ ─ ─ ─ ╯ with gradient
	bottomBorder := renderGradientBottomBorder(innerWidth+2, numColors)
	b.WriteString(bottomBorder)

	return b.String()
}

// renderGradientTopBorder renders the top border with gradient colors
func renderGradientTopBorder(width int, numColors int) string {
	var b strings.Builder

	// Corner uses brightest color
	cornerColor := lipgloss.Color(gradientColors[numColors-1])
	cornerStyle := lipgloss.NewStyle().Foreground(cornerColor)
	b.WriteString(cornerStyle.Render("╭"))

	// Horizontal segments with gradient
	segmentWidth := width / numColors
	if segmentWidth < 1 {
		segmentWidth = 1
	}

	remaining := width
	for i := 0; i < numColors && remaining > 0; i++ {
		color := lipgloss.Color(gradientColors[i])
		style := lipgloss.NewStyle().Foreground(color)
		chars := segmentWidth
		if i == numColors-1 {
			chars = remaining - 1 // Leave room for corner
		}
		if chars > remaining-1 {
			chars = remaining - 1
		}
		if chars > 0 {
			b.WriteString(style.Render(strings.Repeat("─", chars)))
			remaining -= chars
		}
	}

	b.WriteString(cornerStyle.Render("╮"))
	return b.String()
}

// renderGradientBottomBorder renders the bottom border with gradient colors
func renderGradientBottomBorder(width int, numColors int) string {
	var b strings.Builder

	// Corner uses brightest color
	cornerColor := lipgloss.Color(gradientColors[numColors-1])
	cornerStyle := lipgloss.NewStyle().Foreground(cornerColor)
	b.WriteString(cornerStyle.Render("╰"))

	// Horizontal segments with gradient (reverse direction)
	segmentWidth := width / numColors
	if segmentWidth < 1 {
		segmentWidth = 1
	}

	remaining := width
	for i := numColors - 1; i >= 0 && remaining > 0; i-- {
		color := lipgloss.Color(gradientColors[i])
		style := lipgloss.NewStyle().Foreground(color)
		chars := segmentWidth
		if i == 0 {
			chars = remaining - 1 // Leave room for corner
		}
		if chars > remaining-1 {
			chars = remaining - 1
		}
		if chars > 0 {
			b.WriteString(style.Render(strings.Repeat("─", chars)))
			remaining -= chars
		}
	}

	b.WriteString(cornerStyle.Render("╯"))
	return b.String()
}

// RenderAppBackground creates a full-screen background canvas
func RenderAppBackground(content string, width, height int) string {
	bgStyle := lipgloss.NewStyle().
		Background(ColorAppBg).
		Width(width).
		Height(height)

	return bgStyle.Render(content)
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
	currentX := 1 // Start at 1 for left padding

	for _, t := range tabs {
		var rendered string
		if t == activeTab {
			// Active tab with outlined box
			rendered = TabActive.Render(t.String())
		} else {
			// Inactive tab as plain text
			rendered = TabInactive.Render(t.String())
		}
		renderedWidth := lipgloss.Width(rendered)

		positions.Tabs = append(positions.Tabs, TabPosition{
			Tab:   t,
			Start: currentX,
			End:   currentX + renderedWidth,
		})

		currentX += renderedWidth + 1 // +1 for space
		renderedTabs = append(renderedTabs, rendered)
	}

	tabBar := TabBarContainer.Render(strings.Join(renderedTabs, " "))
	divider := TabBarBorder.Render(strings.Repeat("─", width))

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

// StatusBar styles
var (
	StatusBarStyle = lipgloss.NewStyle().
			Background(ColorBgHeader).
			Foreground(ColorMuted).
			Padding(0, 1)

	StatusBarLeftStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	StatusBarRightStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)
)

// RenderStatusBar renders a premium status bar footer
func RenderStatusBar(leftContent, centerHints, rightContent string, width int) string {
	// Calculate widths
	leftWidth := lipgloss.Width(leftContent)
	centerWidth := lipgloss.Width(centerHints)
	rightWidth := lipgloss.Width(rightContent)

	// Calculate gaps for centering
	totalContentWidth := leftWidth + centerWidth + rightWidth
	remainingSpace := width - totalContentWidth - 4

	if remainingSpace < 0 {
		remainingSpace = 0
	}

	leftGap := remainingSpace / 2
	rightGap := remainingSpace - leftGap

	// Build status bar
	bar := StatusBarLeftStyle.Render(leftContent) +
		strings.Repeat(" ", leftGap) +
		centerHints +
		strings.Repeat(" ", rightGap) +
		StatusBarRightStyle.Render(rightContent)

	return StatusBarStyle.Width(width).Render(bar)
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
