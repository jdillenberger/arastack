package handlers

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/pkg/clients"
)

// PeersPagePeer extends a peer with its resolved dashboard URL.
type PeersPagePeer struct {
	clients.Peer
	DashURL string
}

// PeersPageData holds data for the peers template.
type PeersPageData struct {
	BasePage
	Unavailable   bool
	PeerGroupName string
	Self          clients.Peer
	Peers         []PeersPagePeer
}

// HandlePeersPage serves the peers overview HTML page.
func (h *Handler) HandlePeersPage(c echo.Context) error {
	resp, err := h.peerClient.Peers()
	if err != nil {
		return c.Render(http.StatusOK, "peers.html", PeersPageData{
			BasePage:    h.basePage(),
			Unavailable: true,
		})
	}

	requestHost := c.Request().Host
	if idx := strings.LastIndex(requestHost, ":"); idx != -1 {
		requestHost = requestHost[:idx]
	}
	var peers []PeersPagePeer
	for _, p := range resp.Peers {
		peers = append(peers, PeersPagePeer{
			Peer:    p,
			DashURL: peerDashboardURL(p, requestHost),
		})
	}

	data := PeersPageData{
		BasePage:      h.basePage(),
		PeerGroupName: resp.PeerGroup.Name,
		Self:          resp.Self,
		Peers:         peers,
	}

	return c.Render(http.StatusOK, "peers.html", data)
}

// HandlePeersAPI returns peers status as JSON.
func (h *Handler) HandlePeersAPI(c echo.Context) error {
	resp, err := h.peerClient.Peers()
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "arascanner unavailable",
		})
	}

	return c.JSON(http.StatusOK, resp)
}
