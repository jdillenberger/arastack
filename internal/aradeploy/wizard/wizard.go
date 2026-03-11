package wizard

import (
	"fmt"

	"github.com/charmbracelet/huh"

	"github.com/jdillenberger/arastack/internal/aradeploy/code"
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

// RunCodeWizard prompts for code sources when a template has code slots.
// Returns a map of "slot[/name]" to "source[#branch]".
func RunCodeWizard(meta *template.AppMeta) (map[string]string, error) {
	result := make(map[string]string)

	if meta.Code == nil || len(meta.Code.Slots) == 0 {
		return result, nil
	}

	for _, slot := range meta.Code.Slots {
		if slot.Required {
			if err := promptCodeSlot(slot, result); err != nil {
				return nil, err
			}
			continue
		}

		description := slot.Description
		if description == "" {
			description = slot.Name
		}

		var addCode bool
		confirm := huh.NewConfirm().
			Title(fmt.Sprintf("Add custom %s?", description)).
			Value(&addCode)
		if err := confirm.Run(); err != nil {
			return nil, fmt.Errorf("wizard cancelled: %w", err)
		}
		if !addCode {
			continue
		}

		if err := promptCodeSlot(slot, result); err != nil {
			return nil, err
		}

		// For multiple slots, offer to add more
		if slot.Multiple {
			for {
				var addMore bool
				confirm := huh.NewConfirm().
					Title(fmt.Sprintf("Add another %s?", description)).
					Value(&addMore)
				if err := confirm.Run(); err != nil {
					return nil, fmt.Errorf("wizard cancelled: %w", err)
				}
				if !addMore {
					break
				}
				if err := promptCodeSlot(slot, result); err != nil {
					return nil, err
				}
			}
		}
	}

	return result, nil
}

func promptCodeSlot(slot template.CodeSlot, result map[string]string) error {
	var name, source string

	var fields []huh.Field

	if slot.Multiple {
		fields = append(fields, huh.NewInput().
			Title("Name").
			Description(fmt.Sprintf("Name for this %s", slot.Name)).
			Validate(code.ValidateName).
			Value(&name))
	}

	fields = append(fields, huh.NewInput().
		Title("Source").
		Description("Local path or git URL").
		Value(&source))

	form := huh.NewForm(
		huh.NewGroup(fields...).
			Title(fmt.Sprintf("Code: %s", slot.Name)).
			Description(slot.Description),
	)

	if err := form.Run(); err != nil {
		return fmt.Errorf("wizard cancelled: %w", err)
	}

	key := slot.Name
	if name != "" {
		key += "/" + name
	}
	result[key] = source
	return nil
}
