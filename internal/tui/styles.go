package tui

import "github.com/charmbracelet/lipgloss"

type styles struct {
	App              lipgloss.Style
	Header           lipgloss.Style
	SubHeader        lipgloss.Style
	Panel            lipgloss.Style
	PanelFocused     lipgloss.Style
	PanelTitle       lipgloss.Style
	InputLabel       lipgloss.Style
	InputHint        lipgloss.Style
	Footer           lipgloss.Style
	StatusInfo       lipgloss.Style
	StatusSuccess    lipgloss.Style
	StatusWarn       lipgloss.Style
	StatusFail       lipgloss.Style
	OptionCursor     lipgloss.Style
	OptionEnabled    lipgloss.Style
	OptionDisabled   lipgloss.Style
	Muted            lipgloss.Style
	HelpModal        lipgloss.Style
	HelpModalTitle   lipgloss.Style
	LoadingModal     lipgloss.Style
	LoadingSpinner   lipgloss.Style
	ResultHeader     lipgloss.Style
	ResultSummary    lipgloss.Style
	TableHint        lipgloss.Style
	EmptyState       lipgloss.Style
	ErrorText        lipgloss.Style
	BadgeSuccess     lipgloss.Style
	BadgeWarn        lipgloss.Style
	BadgeFail        lipgloss.Style
	SectionTitle     lipgloss.Style
	SectionSeparator lipgloss.Style
}

func newStyles() styles {
	accent := lipgloss.Color("39")
	accentSoft := lipgloss.Color("111")
	warn := lipgloss.Color("214")
	errorColor := lipgloss.Color("204")
	success := lipgloss.Color("42")
	muted := lipgloss.Color("244")
	border := lipgloss.Color("240")

	return styles{
		App: lipgloss.NewStyle().Padding(1, 2),
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			MarginBottom(1),
		SubHeader: lipgloss.NewStyle().
			Foreground(muted).
			MarginBottom(1),
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Padding(1, 1),
		PanelFocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(1, 1),
		PanelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(accentSoft),
		InputLabel: lipgloss.NewStyle().
			Bold(true),
		InputHint: lipgloss.NewStyle().
			Foreground(muted),
		Footer: lipgloss.NewStyle().
			Foreground(muted).
			PaddingTop(1),
		StatusInfo: lipgloss.NewStyle().
			Foreground(accentSoft),
		StatusSuccess: lipgloss.NewStyle().
			Foreground(success).
			Bold(true),
		StatusWarn: lipgloss.NewStyle().
			Foreground(warn).
			Bold(true),
		StatusFail: lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true),
		OptionCursor: lipgloss.NewStyle().
			Foreground(accent).
			Bold(true),
		OptionEnabled: lipgloss.NewStyle().
			Foreground(success),
		OptionDisabled: lipgloss.NewStyle().
			Foreground(muted),
		Muted: lipgloss.NewStyle().
			Foreground(muted),
		HelpModal: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(1, 2).
			Background(lipgloss.Color("236")),
		HelpModalTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			MarginBottom(1),
		LoadingModal: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(accent).
			Padding(1, 2),
		LoadingSpinner: lipgloss.NewStyle().
			Foreground(accent).
			Bold(true),
		ResultHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent),
		ResultSummary: lipgloss.NewStyle().
			Foreground(accentSoft),
		TableHint: lipgloss.NewStyle().
			Foreground(muted).
			Italic(true),
		EmptyState: lipgloss.NewStyle().
			Foreground(muted).
			Italic(true),
		ErrorText: lipgloss.NewStyle().
			Foreground(errorColor),
		BadgeSuccess: lipgloss.NewStyle().
			Background(success).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1).
			Bold(true),
		BadgeWarn: lipgloss.NewStyle().
			Background(warn).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1).
			Bold(true),
		BadgeFail: lipgloss.NewStyle().
			Background(errorColor).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1).
			Bold(true),
		SectionTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent),
		SectionSeparator: lipgloss.NewStyle().
			Foreground(border),
	}
}

func (s styles) outcomeBadge(level outcomeLevel) string {
	switch level {
	case outcomeSuccess:
		return s.BadgeSuccess.Render("SUCCESS")
	case outcomeWarning:
		return s.BadgeWarn.Render("WARN")
	default:
		return s.BadgeFail.Render("ERROR")
	}
}
