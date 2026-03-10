package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jdillenberger/arastack/pkg/clients"
)

const (
	tabOverview   = 0
	tabApps       = 1
	tabContainers = 2
	tabAlerts     = 3
	tabFleet      = 4

	numTabs = 5
)

// Config holds all configuration for the TUI model.
type Config struct {
	MonitorClient *clients.MonitorClient
	AlertClient   *clients.AlertClient
	BackupClient  *clients.BackupClient
	ScannerClient *clients.AraScannerClient

	MonitorURL string
	AlertURL   string
	BackupURL  string
	ScannerURL string

	Interval time.Duration
}

// Model is the bubbletea model for aratop.
type Model struct {
	cfg Config

	// Tab state.
	activeTab int

	// Data: monitor.
	health     []clients.AppHealthResult
	containers []clients.ContainerStatsResult
	monitorErr error

	// Data: system.
	sysInfo SysInfo

	// Data: alerts.
	alertRules   []AlertRule
	alertHistory []AlertHistoryEntry
	alertErr     error

	// Data: backup.
	backupStatus *clients.BackupStatus
	backupErr    error

	// Data: fleet.
	fleetName  string
	fleetSelf  clients.Peer
	fleetPeers []clients.Peer
	fleetErr   error

	// Data: service health.
	serviceHealth map[string]bool

	// Timestamps.
	lastUpdate time.Time

	// UI state.
	width  int
	height int

	// Tables for tabular views.
	appsTable       table.Model
	containersTable table.Model
	alertsTable     table.Model
	fleetTable      table.Model

	// Drill-down state.
	detailView  bool
	detailApp   string
	detailTable table.Model
}

// NewModel creates a new TUI model.
func NewModel(cfg Config) *Model {
	m := &Model{
		cfg:           cfg,
		serviceHealth: make(map[string]bool),
	}
	m.appsTable = newTable()
	m.containersTable = newTable()
	m.alertsTable = newTable()
	m.fleetTable = newTable()
	m.detailTable = newTable()
	return m
}

func newTable() table.Model {
	t := table.New(
		table.WithFocused(true),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorDim).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(colorWhite).
		Background(lipgloss.Color("8")).
		Bold(true)
	t.SetStyles(s)
	return t
}

// Init starts the initial data fetch and tick timers.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		fetchMonitorData(m.cfg.MonitorClient),
		fetchSysInfo(),
		fetchAlertData(m.cfg.AlertClient),
		fetchBackupStatus(m.cfg.BackupClient),
		fetchPeers(m.cfg.ScannerClient),
		fetchServiceHealth(m.cfg),
		tickCmd(m.cfg.Interval),
		slowTickCmd(m.cfg.Interval),
	)
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeTables()
		return m, nil

	case monitorMsg:
		if msg.err != nil {
			m.monitorErr = msg.err
		} else {
			m.monitorErr = nil
		}
		if msg.health != nil {
			m.health = msg.health
		}
		if msg.containers != nil {
			m.containers = msg.containers
		}
		m.lastUpdate = time.Now()
		m.updateAppsTable()
		m.updateContainersTable()
		if m.detailView {
			m.updateDetailTable()
		}
		return m, nil

	case sysInfoMsg:
		m.sysInfo = msg.info
		return m, nil

	case alertMsg:
		if msg.err != nil {
			m.alertErr = msg.err
		} else {
			m.alertErr = nil
			m.alertRules = msg.rules
			m.alertHistory = msg.history
		}
		m.updateAlertsTable()
		return m, nil

	case backupMsg:
		if msg.err != nil {
			m.backupErr = msg.err
		} else {
			m.backupErr = nil
			m.backupStatus = msg.status
		}
		return m, nil

	case peersMsg:
		if msg.err != nil {
			m.fleetErr = msg.err
		} else {
			m.fleetErr = nil
			m.fleetName = msg.fleet
			m.fleetSelf = msg.self
			m.fleetPeers = msg.peers
		}
		m.updateFleetTable()
		return m, nil

	case serviceHealthMsg:
		m.serviceHealth = msg.results
		return m, nil

	case tickMsg:
		return m, tea.Batch(
			fetchMonitorData(m.cfg.MonitorClient),
			fetchSysInfo(),
			tickCmd(m.cfg.Interval),
		)

	case slowTickMsg:
		return m, tea.Batch(
			fetchAlertData(m.cfg.AlertClient),
			fetchBackupStatus(m.cfg.BackupClient),
			fetchPeers(m.cfg.ScannerClient),
			fetchServiceHealth(m.cfg),
			slowTickCmd(m.cfg.Interval),
		)
	}

	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If in detail view, handle Esc first.
	if m.detailView {
		if key.Matches(msg, keys.Back) {
			m.detailView = false
			return m, nil
		}
		// Forward to detail table.
		var cmd tea.Cmd
		m.detailTable, cmd = m.detailTable.Update(msg)
		return m, cmd
	}

	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, keys.Tab1):
		m.activeTab = tabOverview
	case key.Matches(msg, keys.Tab2):
		m.activeTab = tabApps
	case key.Matches(msg, keys.Tab3):
		m.activeTab = tabContainers
	case key.Matches(msg, keys.Tab4):
		m.activeTab = tabAlerts
	case key.Matches(msg, keys.Tab5):
		m.activeTab = tabFleet

	case key.Matches(msg, keys.NextTab):
		m.activeTab = (m.activeTab + 1) % numTabs
	case key.Matches(msg, keys.PrevTab):
		m.activeTab = (m.activeTab - 1 + numTabs) % numTabs

	case key.Matches(msg, keys.Refresh):
		return m, tea.Batch(
			fetchMonitorData(m.cfg.MonitorClient),
			fetchSysInfo(),
			fetchAlertData(m.cfg.AlertClient),
			fetchBackupStatus(m.cfg.BackupClient),
			fetchPeers(m.cfg.ScannerClient),
			fetchServiceHealth(m.cfg),
		)

	case key.Matches(msg, keys.Enter):
		if m.activeTab == tabApps && len(m.health) > 0 {
			sel := m.appsTable.Cursor()
			if sel >= 0 && sel < len(m.health) {
				m.detailApp = m.health[sel].App
				m.detailView = true
				m.updateDetailTable()
				return m, nil
			}
		}

	default:
		// Forward key to active table.
		return m.forwardToTable(msg)
	}

	return m, nil
}

func (m *Model) forwardToTable(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.activeTab {
	case tabApps:
		m.appsTable, cmd = m.appsTable.Update(msg)
	case tabContainers:
		m.containersTable, cmd = m.containersTable.Update(msg)
	case tabAlerts:
		m.alertsTable, cmd = m.alertsTable.Update(msg)
	case tabFleet:
		m.fleetTable, cmd = m.fleetTable.Update(msg)
	}
	return m, cmd
}

func (m *Model) resizeTables() {
	w := m.width - 4  // padding
	h := m.height - 6 // tab bar + status bar + borders
	if h < 3 {
		h = 3
	}

	m.appsTable.SetWidth(w)
	m.appsTable.SetHeight(h)
	m.containersTable.SetWidth(w)
	m.containersTable.SetHeight(h)
	m.alertsTable.SetWidth(w)
	m.alertsTable.SetHeight(h)
	m.fleetTable.SetWidth(w)
	m.fleetTable.SetHeight(h)
	m.detailTable.SetWidth(w)
	m.detailTable.SetHeight(h)
}

// View renders the TUI.
func (m *Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var content string

	if m.detailView {
		content = renderAppDetail(m)
	} else {
		switch m.activeTab {
		case tabOverview:
			content = renderOverview(m)
		case tabApps:
			content = renderAppsView(m)
		case tabContainers:
			content = renderContainersView(m)
		case tabAlerts:
			content = renderAlertsView(m)
		case tabFleet:
			content = renderFleetView(m)
		}
	}

	// Compose: tab bar + error + content + status bar.
	var sections []string

	updateStr := ""
	if !m.lastUpdate.IsZero() {
		updateStr = m.lastUpdate.Format("15:04:05")
	}
	sections = append(sections, renderTabBar(m.activeTab, m.width, updateStr))

	if m.monitorErr != nil {
		sections = append(sections, renderErrorBanner(m.monitorErr, m.width))
	}

	sections = append(sections, content)

	// Help bar.
	var hints string
	if m.detailView {
		hints = "  esc back | j/k navigate | r refresh | q quit"
	} else {
		switch m.activeTab {
		case tabOverview:
			hints = "  1-5 tab | tab cycle | r refresh | q quit"
		case tabApps:
			hints = "  1-5 tab | j/k navigate | enter detail | r refresh | q quit"
		default:
			hints = "  1-5 tab | j/k navigate | r refresh | q quit"
		}
	}
	sections = append(sections, renderStatusBar(hints, m.width))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
