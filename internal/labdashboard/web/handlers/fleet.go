package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/pkg/clients"
)

// FleetPageData holds data for the fleet template.
type FleetPageData struct {
	BasePage
	Unavailable bool
	FleetName   string
	Self        clients.Peer
	Peers       []clients.Peer
}

// HandleFleetPage serves the fleet overview HTML page.
func (h *Handler) HandleFleetPage(c echo.Context) error {
	resp, err := h.peerClient.Peers()
	if err != nil {
		return c.Render(http.StatusOK, "fleet.html", FleetPageData{
			BasePage:    h.basePage(),
			Unavailable: true,
		})
	}

	data := FleetPageData{
		BasePage:  h.basePage(),
		FleetName: resp.Fleet.Name,
		Self:      resp.Self,
		Peers:     resp.Peers,
	}

	return c.Render(http.StatusOK, "fleet.html", data)
}

// HandleFleetAPI returns fleet status as JSON.
func (h *Handler) HandleFleetAPI(c echo.Context) error {
	resp, err := h.peerClient.Peers()
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "peer-scanner unavailable",
		})
	}

	return c.JSON(http.StatusOK, resp)
}
