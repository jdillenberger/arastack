package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/pkg/clients"
)

// PeersPageData holds data for the peers template.
type PeersPageData struct {
	BasePage
	Unavailable   bool
	PeerGroupName string
	Self          clients.Peer
	Peers         []clients.Peer
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

	data := PeersPageData{
		BasePage:      h.basePage(),
		PeerGroupName: resp.PeerGroup.Name,
		Self:          resp.Self,
		Peers:         resp.Peers,
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
