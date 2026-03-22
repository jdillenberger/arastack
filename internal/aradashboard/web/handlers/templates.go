package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/yuin/goldmark"

	apptmpl "github.com/jdillenberger/arastack/internal/aradeploy/template"
)

// TemplateSummary holds the fields shown in the templates list.
type TemplateSummary struct {
	Name        string
	Description string
	Category    string
	Version     string
}

// TemplatesListData holds data for the templates list template.
type TemplatesListData struct {
	BasePage
	Templates []TemplateSummary
}

// TemplateDetailData holds data for the template detail template.
type TemplateDetailData struct {
	BasePage
	Template  *apptmpl.AppMeta
	Values    []apptmpl.Value // non-secret values only
	HasValues bool
	ReadmeHTML template.HTML
}

// TemplatesList renders the available templates page.
func (h *Handler) TemplatesList(c echo.Context) error {
	data := TemplatesListData{
		BasePage: h.basePage(),
	}

	if h.registry != nil {
		for _, meta := range h.registry.All() {
			data.Templates = append(data.Templates, TemplateSummary{
				Name:        meta.Name,
				Description: meta.Description,
				Category:    meta.Category,
				Version:     meta.Version,
			})
		}
	}

	return c.Render(http.StatusOK, "templates.html", data)
}

// TemplateDetail renders the template detail page.
func (h *Handler) TemplateDetail(c echo.Context) error {
	name := c.Param("name")

	if h.registry == nil {
		return echo.NewHTTPError(http.StatusNotFound, "templates not available")
	}

	meta, ok := h.registry.Get(name)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("template %s not found", name))
	}

	var publicValues []apptmpl.Value
	for _, v := range meta.Values {
		if !v.Secret {
			publicValues = append(publicValues, v)
		}
	}

	data := TemplateDetailData{
		BasePage:  h.basePage(),
		Template:  meta,
		Values:    publicValues,
		HasValues: len(publicValues) > 0,
	}

	if h.registry.FS() != nil {
		if md, err := fs.ReadFile(h.registry.FS(), name+"/README.md"); err == nil {
			var buf bytes.Buffer
			if err := goldmark.Convert(md, &buf); err == nil {
				data.ReadmeHTML = template.HTML(buf.Bytes()) // #nosec G203 -- rendered from trusted template README
			}
		}
	}

	return c.Render(http.StatusOK, "template_detail.html", data)
}
