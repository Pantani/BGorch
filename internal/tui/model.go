package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Pantani/gorchestrator/internal/app"
)

type screen int

const (
	screenDashboard screen = iota
	screenResult
)

type dashboardFocus int

const (
	focusActions dashboardFocus = iota
	focusSpecPath
	focusOutputDir
	focusOptions
)

type resultFocus int

const (
	resultFocusSummary resultFocus = iota
	resultFocusTable
)

type tableKind int

const (
	tableKindNone tableKind = iota
	tableKindPlan
	tableKindDoctor
	tableKindDiagnostics
)

type actionFinishedMsg struct {
	result actionResult
}

type model struct {
	width  int
	height int

	screen   screen
	helpOpen bool
	running  bool

	styles styles
	keys   keyMap
	help   help.Model

	actions     list.Model
	specInput   textinput.Model
	outputInput textinput.Model
	spinner     spinner.Model

	focus        dashboardFocus
	optionCursor int
	config       runConfig

	runner runner

	statusLine  string
	statusLevel outcomeLevel
	confirmOpen bool

	result          actionResult
	resultViewport  viewport.Model
	resultTable     table.Model
	resultTableKind tableKind
	resultFocus     resultFocus
}

func New(application *app.App) model {
	uiStyles := newStyles()
	keys := newKeyMap()
	helpModel := help.New()
	helpModel.ShowAll = false

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	actions := list.New(allActions(), delegate, 30, 20)
	actions.Title = "Actions"
	actions.SetShowHelp(false)
	actions.SetShowPagination(false)
	actions.SetShowStatusBar(false)
	actions.SetFilteringEnabled(true)
	actions.DisableQuitKeybindings()

	specInput := textinput.New()
	specInput.Prompt = ""
	specInput.Placeholder = "examples/generic-single-compose.yaml"
	specInput.CharLimit = 0

	outputInput := textinput.New()
	outputInput.Prompt = ""
	outputInput.Placeholder = ".bgorch/render"
	outputInput.CharLimit = 0
	outputInput.SetValue(".bgorch/render")

	spin := spinner.New()
	spin.Spinner = spinner.Dot

	viewportModel := viewport.New(10, 10)
	viewportModel.SetContent("Run an action to see details.")

	resultTable := table.New(
		table.WithColumns([]table.Column{
			{Title: "Type", Width: 12},
			{Title: "Resource", Width: 20},
			{Title: "Name", Width: 22},
			{Title: "Reason", Width: 32},
		}),
		table.WithRows(nil),
		table.WithHeight(8),
		table.WithFocused(true),
	)
	resultTable.SetStyles(tableStyles())

	m := model{
		screen:          screenDashboard,
		helpOpen:        false,
		running:         false,
		styles:          uiStyles,
		keys:            keys,
		help:            helpModel,
		actions:         actions,
		specInput:       specInput,
		outputInput:     outputInput,
		spinner:         spin,
		focus:           focusActions,
		optionCursor:    0,
		config:          runConfig{OutputDir: ".bgorch/render"},
		runner:          newRunner(application),
		statusLine:      "Select an action and configure the form.",
		statusLevel:     outcomeSuccess,
		resultViewport:  viewportModel,
		resultTable:     resultTable,
		resultTableKind: tableKindNone,
		resultFocus:     resultFocusSummary,
	}
	m.applyDashboardFocus()
	return m
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		m.resize()
	case actionFinishedMsg:
		m.running = false
		m.result = typed.result
		m.screen = screenResult
		m.resultFocus = resultFocusSummary
		m.statusLine = typed.result.Summary
		m.statusLevel = typed.result.Outcome
		m.prepareResultView()
		return m, nil
	}

	if m.running {
		switch typed := msg.(type) {
		case spinner.TickMsg:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(typed)
			return m, cmd
		case tea.KeyMsg:
			if key.Matches(typed, m.keys.Help, m.keys.Back) {
				m.helpOpen = !m.helpOpen
				return m, nil
			}
			if key.Matches(typed, m.keys.Quit) {
				return m, tea.Quit
			}
		}
		return m, nil
	}

	if typed, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(typed, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(typed, m.keys.Help):
			m.helpOpen = !m.helpOpen
			return m, nil
		case key.Matches(typed, m.keys.Back) && m.helpOpen:
			m.helpOpen = false
			return m, nil
		}
	}

	if m.helpOpen {
		return m, nil
	}

	switch m.screen {
	case screenDashboard:
		m, cmds = m.updateDashboard(msg)
	case screenResult:
		m, cmds = m.updateResult(msg)
	}

	if m.running {
		cmds = append(cmds, m.spinner.Tick)
	}

	return m, tea.Batch(cmds...)
}

func (m model) updateDashboard(msg tea.Msg) (model, []tea.Cmd) {
	cmds := make([]tea.Cmd, 0, 3)
	beforeAction := m.selectedAction()

	if m.focus == focusActions {
		var cmd tea.Cmd
		m.actions, cmd = m.actions.Update(msg)
		cmds = append(cmds, cmd)
	}

	if beforeAction != m.selectedAction() {
		m.optionCursor = 0
		m.confirmOpen = false
		if !m.currentFocusAvailable() {
			m.focus = focusActions
		}
		m.applyDashboardFocus()
		m.resize()
	}

	if m.confirmOpen {
		if typed, ok := msg.(tea.KeyMsg); ok {
			switch {
			case key.Matches(typed, m.keys.Select):
				action := m.selectedAction()
				cfg := m.config
				m.confirmOpen = false
				m.statusLine = "Running " + string(action) + "..."
				m.statusLevel = outcomeSuccess
				m.running = true
				cmds = append(cmds, runActionCmd(m.runner, action, cfg))
			case key.Matches(typed, m.keys.Back):
				m.confirmOpen = false
				m.statusLine = "Apply canceled."
				m.statusLevel = outcomeWarning
			}
		}
		return m, cmds
	}

	switch m.focus {
	case focusSpecPath:
		var cmd tea.Cmd
		m.specInput, cmd = m.specInput.Update(msg)
		m.config.SpecPath = m.specInput.Value()
		cmds = append(cmds, cmd)
	case focusOutputDir:
		if actionUsesOutputDir(m.selectedAction()) {
			var cmd tea.Cmd
			m.outputInput, cmd = m.outputInput.Update(msg)
			m.config.OutputDir = m.outputInput.Value()
			cmds = append(cmds, cmd)
		}
	}

	if typed, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(typed, m.keys.Run):
			nextModel, cmd := m.startSelectedAction()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return nextModel, cmds
		case key.Matches(typed, m.keys.Select) && m.focus == focusActions:
			if !m.isFilteringActions() {
				nextModel, cmd := m.startSelectedAction()
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				return nextModel, cmds
			}
		case key.Matches(typed, m.keys.NextFocus):
			m.focus = m.nextFocus()
			m.applyDashboardFocus()
			m.resize()
		case key.Matches(typed, m.keys.PrevFocus):
			m.focus = m.prevFocus()
			m.applyDashboardFocus()
			m.resize()
		case key.Matches(typed, m.keys.Select), key.Matches(typed, m.keys.Toggle):
			if m.focus == focusOptions {
				m.toggleCurrentOption()
			}
		case key.Matches(typed, m.keys.Up) && m.focus == focusOptions:
			if m.optionCursor > 0 {
				m.optionCursor--
			}
		case key.Matches(typed, m.keys.Down) && m.focus == focusOptions:
			options := actionOptions(m.selectedAction())
			if m.optionCursor < len(options)-1 {
				m.optionCursor++
			}
		case key.Matches(typed, m.keys.Back):
			m.focus = focusActions
			m.applyDashboardFocus()
		}
	}

	return m, cmds
}

func (m model) updateResult(msg tea.Msg) (model, []tea.Cmd) {
	cmds := make([]tea.Cmd, 0, 2)

	if typed, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(typed, m.keys.Back):
			m.screen = screenDashboard
			m.resultFocus = resultFocusSummary
			return m, cmds
		case key.Matches(typed, m.keys.NextFocus):
			if m.resultTableKind != tableKindNone {
				if m.resultFocus == resultFocusSummary {
					m.resultFocus = resultFocusTable
				} else {
					m.resultFocus = resultFocusSummary
				}
			}
			return m, cmds
		}
	}

	if m.resultFocus == resultFocusTable && m.resultTableKind != tableKindNone {
		var cmd tea.Cmd
		m.resultTable, cmd = m.resultTable.Update(msg)
		cmds = append(cmds, cmd)
		return m, cmds
	}

	var cmd tea.Cmd
	m.resultViewport, cmd = m.resultViewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, cmds
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}

	if m.helpOpen {
		return m.renderHelpView()
	}

	base := ""
	switch m.screen {
	case screenDashboard:
		base = m.renderDashboard()
	case screenResult:
		base = m.renderResult()
	}

	if m.running {
		loadingLine := m.styles.LoadingSpinner.Render(m.spinner.View() + " Running " + string(m.selectedAction()) + "...")
		base += "\n" + loadingLine
	}

	return m.styles.App.Width(m.width).Height(m.height).Render(base)
}

func (m *model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}

	contentWidth := max(40, m.width-6)
	contentHeight := max(15, m.height-8)

	leftWidth := max(30, contentWidth/2)
	rightWidth := max(28, contentWidth-leftWidth-1)
	if contentWidth < 100 {
		leftWidth = contentWidth
		rightWidth = contentWidth
	}

	m.actions.SetSize(max(20, leftWidth-4), max(8, contentHeight-3))
	m.specInput.Width = max(20, rightWidth-4)
	m.outputInput.Width = max(20, rightWidth-4)

	resultWidth := max(30, contentWidth)
	resultHeight := max(10, contentHeight)
	if m.resultTableKind == tableKindNone {
		m.resultViewport.Width = resultWidth - 4
		m.resultViewport.Height = resultHeight - 4
		return
	}

	if contentWidth < 100 {
		m.resultViewport.Width = resultWidth - 4
		m.resultViewport.Height = max(6, (resultHeight/2)-2)
		m.resultTable.SetWidth(resultWidth - 4)
		m.resultTable.SetHeight(max(4, (resultHeight/2)-3))
		return
	}

	summaryWidth := max(30, (resultWidth/2)-2)
	tableWidth := max(30, resultWidth-summaryWidth-1)
	m.resultViewport.Width = summaryWidth
	m.resultViewport.Height = resultHeight - 4
	m.resultTable.SetWidth(tableWidth)
	m.resultTable.SetHeight(resultHeight - 4)
}

func (m model) renderDashboard() string {
	action := m.selectedAction()
	header := m.styles.Header.Render("BGorch TUI")
	sub := m.styles.SubHeader.Render("Bubble Tea dashboard for validate/render/plan/apply/status/doctor")

	leftPanelStyle := m.styles.Panel
	rightPanelStyle := m.styles.Panel
	if m.focus == focusActions {
		leftPanelStyle = m.styles.PanelFocused
	}
	if m.focus != focusActions {
		rightPanelStyle = m.styles.PanelFocused
	}

	leftPanel := leftPanelStyle.Render(strings.Join([]string{
		m.styles.PanelTitle.Render("Commands"),
		m.actions.View(),
	}, "\n"))

	formBlocks := []string{
		m.styles.PanelTitle.Render("Execution Form"),
		m.renderSpecInput(),
		m.renderOutputInput(action),
		m.renderOptions(action),
		m.renderRunHint(action),
	}
	if m.confirmOpen {
		formBlocks = append(formBlocks, m.renderConfirmDialog())
	}
	rightPanel := rightPanelStyle.Render(strings.Join(formBlocks, "\n\n"))

	var body string
	if m.width < 100 {
		body = lipgloss.JoinVertical(lipgloss.Left, leftPanel, rightPanel)
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	}

	footer := m.renderFooter()
	status := m.renderStatusLine()

	return strings.Join([]string{header, sub, body, status, footer}, "\n")
}

func (m model) renderSpecInput() string {
	label := m.styles.InputLabel.Render("Spec file")
	hint := m.styles.InputHint.Render("Path to cluster spec YAML")
	return strings.Join([]string{label, m.specInput.View(), hint}, "\n")
}

func (m model) renderOutputInput(action actionID) string {
	label := m.styles.InputLabel.Render("Output directory")
	if !actionUsesOutputDir(action) {
		return strings.Join([]string{
			label,
			m.styles.Muted.Render(m.outputInput.Value()),
			m.styles.InputHint.Render("Not used by this action"),
		}, "\n")
	}
	return strings.Join([]string{
		label,
		m.outputInput.View(),
		m.styles.InputHint.Render("Used by render/apply/status/doctor"),
	}, "\n")
}

func (m model) renderOptions(action actionID) string {
	opts := actionOptions(action)
	if len(opts) == 0 {
		return m.styles.Muted.Render("No optional flags for this action.")
	}

	lines := []string{m.styles.InputLabel.Render("Options")}
	for idx, option := range opts {
		cursor := " "
		if m.focus == focusOptions && idx == m.optionCursor {
			cursor = m.styles.OptionCursor.Render("▸")
		}

		enabled := m.optionValue(option.field)
		stateLabel := "OFF"
		stateStyle := m.styles.OptionDisabled
		if enabled {
			stateLabel = "ON"
			stateStyle = m.styles.OptionEnabled
		}

		lines = append(lines, fmt.Sprintf("%s [%s] %s", cursor, stateStyle.Render(stateLabel), option.label))
		lines = append(lines, m.styles.InputHint.Render("   "+option.hint))
	}
	lines = append(lines, m.styles.InputHint.Render("Use ↑/↓ and space to toggle."))
	return strings.Join(lines, "\n")
}

func (m model) renderRunHint(action actionID) string {
	hint := fmt.Sprintf("enter/ctrl+r: run %s", action)
	if m.confirmOpen {
		hint = "confirmation required for apply"
	}
	if m.running {
		hint = "running... inputs disabled"
	}
	return m.styles.ResultSummary.Render(hint)
}

func (m model) renderConfirmDialog() string {
	lines := []string{
		m.styles.StatusWarn.Render("Confirm apply"),
		"This apply is not a dry-run and will mutate artifacts/state.",
		m.styles.InputHint.Render("Press enter to confirm or esc to cancel."),
	}
	return m.styles.PanelFocused.Render(strings.Join(lines, "\n"))
}

func (m model) renderStatusLine() string {
	line := m.statusLine
	if strings.TrimSpace(line) == "" {
		line = "Ready"
	}
	switch m.statusLevel {
	case outcomeSuccess:
		return m.styles.StatusSuccess.Render(line)
	case outcomeWarning:
		return m.styles.StatusWarn.Render(line)
	default:
		return m.styles.StatusFail.Render(line)
	}
}

func (m model) renderFooter() string {
	return m.styles.Footer.Render(m.help.View(m.keys))
}

func (m model) renderResult() string {
	header := m.styles.ResultHeader.Render(strings.ToUpper(string(m.result.Action)) + " result")
	badge := m.styles.outcomeBadge(m.result.Outcome)
	summary := m.styles.ResultSummary.Render(m.result.Summary + " · " + m.result.Duration.Truncate(1e6).String())

	summaryPanel := m.styles.Panel.Render(strings.Join([]string{
		m.styles.PanelTitle.Render("Summary"),
		m.resultViewport.View(),
	}, "\n"))

	resultBody := summaryPanel
	if m.resultTableKind != tableKindNone {
		tablePanel := m.styles.Panel.Render(strings.Join([]string{
			m.styles.PanelTitle.Render(m.tableTitle()),
			m.resultTable.View(),
			m.styles.TableHint.Render(m.tableSelectionHint()),
		}, "\n"))

		if m.width < 100 {
			resultBody = lipgloss.JoinVertical(lipgloss.Left, summaryPanel, tablePanel)
		} else {
			resultBody = lipgloss.JoinHorizontal(lipgloss.Top, summaryPanel, tablePanel)
		}
	}

	if m.resultTableKind == tableKindNone {
		empty := m.styles.EmptyState.Render("No tabular data for this action.")
		resultBody += "\n" + empty
	}

	footer := m.styles.Footer.Render("esc: back · tab: switch summary/table · q: quit · ?: help")
	if m.result.Err != nil {
		footer = m.styles.ErrorText.Render(m.result.Err.Error()) + "\n" + footer
	}

	return strings.Join([]string{lipgloss.JoinHorizontal(lipgloss.Top, badge, " "+header), summary, resultBody, footer}, "\n")
}

func (m model) renderHelpView() string {
	content := []string{
		m.styles.HelpModalTitle.Render("BGorch TUI Help"),
		"Dashboard",
		"- ↑/↓ or j/k: navigate actions and option list",
		"- /: filter actions",
		"- tab / shift+tab: move focus",
		"- space: toggle focused option",
		"- enter or ctrl+r: run selected action",
		"- esc: return focus to action list",
		"- apply (non dry-run): requires explicit confirmation",
		"",
		"Result",
		"- tab: switch focus between summary and table",
		"- esc: back to dashboard",
		"",
		"Global",
		"- ?: toggle help",
		"- q or ctrl+c: quit",
		"",
		m.styles.Muted.Render("Press ? or esc to return."),
	}
	view := strings.Join(content, "\n")
	return m.styles.App.Width(m.width).Height(m.height).Render(m.styles.HelpModal.Render(view))
}

func (m model) selectedAction() actionID {
	item, ok := m.actions.SelectedItem().(actionItem)
	if !ok {
		return actionValidate
	}
	return item.id
}

func (m model) isFilteringActions() bool {
	return m.actions.FilterState() == list.Filtering
}

func (m model) startSelectedAction() (model, tea.Cmd) {
	if m.running {
		return m, nil
	}

	m.config.SpecPath = strings.TrimSpace(m.specInput.Value())
	m.config.OutputDir = strings.TrimSpace(m.outputInput.Value())
	if m.config.OutputDir == "" {
		m.config.OutputDir = ".bgorch/render"
		m.outputInput.SetValue(m.config.OutputDir)
	}

	if err := m.config.validateFor(m.selectedAction()); err != nil {
		m.statusLevel = outcomeFailure
		m.statusLine = err.Error()
		return m, nil
	}

	action := m.selectedAction()
	if action == actionApply && !m.config.DryRun {
		m.confirmOpen = true
		m.statusLine = "Apply will write artifacts/snapshot. Press enter to confirm or esc to cancel."
		m.statusLevel = outcomeWarning
		return m, nil
	}
	m.statusLine = "Running " + string(action) + "..."
	m.statusLevel = outcomeSuccess
	m.running = true
	return m, runActionCmd(m.runner, action, m.config)
}

func (m model) nextFocus() dashboardFocus {
	targets := m.availableFocusTargets()
	idx := indexOfFocus(targets, m.focus)
	return targets[(idx+1)%len(targets)]
}

func (m model) prevFocus() dashboardFocus {
	targets := m.availableFocusTargets()
	idx := indexOfFocus(targets, m.focus)
	if idx == 0 {
		return targets[len(targets)-1]
	}
	return targets[idx-1]
}

func (m model) availableFocusTargets() []dashboardFocus {
	targets := []dashboardFocus{focusActions, focusSpecPath}
	if actionUsesOutputDir(m.selectedAction()) {
		targets = append(targets, focusOutputDir)
	}
	if len(actionOptions(m.selectedAction())) > 0 {
		targets = append(targets, focusOptions)
	}
	return targets
}

func (m model) currentFocusAvailable() bool {
	for _, focusTarget := range m.availableFocusTargets() {
		if focusTarget == m.focus {
			return true
		}
	}
	return false
}

func (m *model) applyDashboardFocus() {
	m.specInput.Blur()
	m.outputInput.Blur()

	switch m.focus {
	case focusSpecPath:
		m.specInput.Focus()
	case focusOutputDir:
		if actionUsesOutputDir(m.selectedAction()) {
			m.outputInput.Focus()
		}
	}
}

func (m *model) toggleCurrentOption() {
	options := actionOptions(m.selectedAction())
	if len(options) == 0 {
		return
	}
	if m.optionCursor < 0 {
		m.optionCursor = 0
	}
	if m.optionCursor >= len(options) {
		m.optionCursor = len(options) - 1
	}

	switch options[m.optionCursor].field {
	case optionWriteState:
		m.config.WriteState = !m.config.WriteState
	case optionDryRun:
		m.config.DryRun = !m.config.DryRun
		if m.config.DryRun && m.config.RuntimeExec {
			m.config.RuntimeExec = false
			m.statusLine = "Dry-run enabled; runtime exec was disabled to keep flags valid."
			m.statusLevel = outcomeWarning
		}
	case optionRuntimeExec:
		m.config.RuntimeExec = !m.config.RuntimeExec
		if m.config.RuntimeExec && m.config.DryRun {
			m.config.DryRun = false
			m.statusLine = "Runtime exec enabled; dry-run was disabled to keep flags valid."
			m.statusLevel = outcomeWarning
		}
	case optionObserveRuntime:
		m.config.ObserveRuntime = !m.config.ObserveRuntime
	}
}

func (m model) optionValue(field optionField) bool {
	switch field {
	case optionWriteState:
		return m.config.WriteState
	case optionDryRun:
		return m.config.DryRun
	case optionRuntimeExec:
		return m.config.RuntimeExec
	case optionObserveRuntime:
		return m.config.ObserveRuntime
	default:
		return false
	}
}

func (m *model) prepareResultView() {
	m.resultViewport.SetContent(m.resultSummaryText())
	m.resultViewport.GotoTop()
	m.buildResultTable()
	m.resize()
}

func (m model) resultSummaryText() string {
	parts := make([]string, 0, len(m.result.Sections)+2)
	parts = append(parts, fmt.Sprintf("Action: %s", strings.ToUpper(string(m.result.Action))))
	parts = append(parts, fmt.Sprintf("Duration: %s", m.result.Duration.Round(1e6)))

	for _, section := range m.result.Sections {
		parts = append(parts, "")
		parts = append(parts, m.styles.SectionTitle.Render(section.Title))
		parts = append(parts, section.Lines...)
	}

	if m.result.Err != nil {
		parts = append(parts, "")
		parts = append(parts, m.styles.ErrorText.Render("Error: "+m.result.Err.Error()))
	}

	return strings.Join(parts, "\n")
}

func (m *model) buildResultTable() {
	if len(m.result.PlanChanges) > 0 {
		columns := []table.Column{
			{Title: "Type", Width: 10},
			{Title: "Resource", Width: 16},
			{Title: "Name", Width: 24},
			{Title: "Reason", Width: 40},
		}
		rows := make([]table.Row, 0, len(m.result.PlanChanges))
		for _, change := range m.result.PlanChanges {
			rows = append(rows, table.Row{
				strings.ToUpper(string(change.Type)),
				change.ResourceType,
				change.Name,
				fallback(change.Reason, "-"),
			})
		}
		m.resultTableKind = tableKindPlan
		m.resultTable.SetColumns(columns)
		m.resultTable.SetRows(rows)
		m.resultTable.GotoTop()
		return
	}

	if len(m.result.DoctorChecks) > 0 {
		columns := []table.Column{
			{Title: "Status", Width: 10},
			{Title: "Check", Width: 26},
			{Title: "Message", Width: 44},
		}
		rows := make([]table.Row, 0, len(m.result.DoctorChecks))
		for _, check := range m.result.DoctorChecks {
			rows = append(rows, table.Row{
				strings.ToUpper(string(check.Status)),
				check.Name,
				check.Message,
			})
		}
		m.resultTableKind = tableKindDoctor
		m.resultTable.SetColumns(columns)
		m.resultTable.SetRows(rows)
		m.resultTable.GotoTop()
		return
	}

	if len(m.result.Diagnostics) > 0 {
		columns := []table.Column{
			{Title: "Severity", Width: 10},
			{Title: "Path", Width: 24},
			{Title: "Message", Width: 46},
		}
		rows := make([]table.Row, 0, len(m.result.Diagnostics))
		for _, diagnostic := range m.result.Diagnostics {
			msg := diagnostic.Message
			if strings.TrimSpace(diagnostic.Hint) != "" {
				msg = msg + " | hint: " + diagnostic.Hint
			}
			rows = append(rows, table.Row{
				strings.ToUpper(string(diagnostic.Severity)),
				fallback(diagnostic.Path, "-"),
				msg,
			})
		}
		m.resultTableKind = tableKindDiagnostics
		m.resultTable.SetColumns(columns)
		m.resultTable.SetRows(rows)
		m.resultTable.GotoTop()
		return
	}

	m.resultTableKind = tableKindNone
	m.resultTable.SetRows(nil)
}

func (m model) tableTitle() string {
	switch m.resultTableKind {
	case tableKindPlan:
		return "Plan Changes"
	case tableKindDoctor:
		return "Doctor Checks"
	case tableKindDiagnostics:
		return "Diagnostics"
	default:
		return "Data"
	}
}

func (m model) tableSelectionHint() string {
	if m.resultTableKind == tableKindNone {
		return ""
	}
	row := m.resultTable.SelectedRow()
	if len(row) == 0 {
		return "Use ↑/↓ to navigate rows."
	}

	switch m.resultTableKind {
	case tableKindPlan:
		return fmt.Sprintf("%s %s (%s)", row[0], row[2], row[3])
	case tableKindDoctor:
		return fmt.Sprintf("%s: %s", row[1], row[2])
	case tableKindDiagnostics:
		return fmt.Sprintf("%s: %s", row[1], row[2])
	default:
		return ""
	}
}

func tableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	styles.Selected = styles.Selected.
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("33")).
		Bold(true)
	return styles
}

func indexOfFocus(targets []dashboardFocus, target dashboardFocus) int {
	for idx, candidate := range targets {
		if candidate == target {
			return idx
		}
	}
	return 0
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func runActionCmd(r runner, action actionID, cfg runConfig) tea.Cmd {
	return func() tea.Msg {
		result := r.run(context.Background(), action, cfg)
		return actionFinishedMsg{result: result}
	}
}

var _ tea.Model = model{}
