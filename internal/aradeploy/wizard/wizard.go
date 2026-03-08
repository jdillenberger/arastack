package wizard

import (
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/jdillenberger/arastack/internal/aradeploy/template"
)

// RunDeployWizard runs an interactive form for configuring app values.
func RunDeployWizard(meta *template.AppMeta) (map[string]string, error) {
	values := make(map[string]string)

	type fieldBinding struct {
		name  string
		value string
	}
	bindings := make([]*fieldBinding, 0, len(meta.Values))

	var fields []huh.Field
	for _, v := range meta.Values {
		if v.Secret && v.AutoGen != "" {
			continue
		}

		description := v.Description
		if v.Required {
			description += " (required)"
		}
		if v.Default != "" {
			description += fmt.Sprintf(" [default: %s]", v.Default)
		}
		if v.Secret {
			description += " [secret]"
		}

		b := &fieldBinding{name: v.Name, value: v.Default}
		bindings = append(bindings, b)
		fields = append(fields, huh.NewInput().
			Title(v.Name).
			Description(description).
			Value(&b.value))
	}

	if len(fields) == 0 {
		return values, nil
	}

	form := huh.NewForm(
		huh.NewGroup(fields...).
			Title(fmt.Sprintf("Configure %s", meta.Name)).
			Description(meta.Description),
	)

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("wizard cancelled: %w", err)
	}

	for _, b := range bindings {
		values[b.name] = b.value
	}

	return values, nil
}
