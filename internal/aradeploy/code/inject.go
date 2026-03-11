package code

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jdillenberger/arastack/internal/aradeploy/template"
	"github.com/jdillenberger/arastack/pkg/executil"
	"gopkg.in/yaml.v3"
)

// InjectCodeVolumes adds bind mount volumes for "volume" code slots into the compose YAML.
// Only processes slots with inject=volume. Pattern follows routing/traefik.go InjectLabels.
func InjectCodeVolumes(composeYAML string, slots []template.CodeSlot, sources []Source, codeDir, appName string) (string, error) {
	if len(sources) == 0 {
		return composeYAML, nil
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil {
		return "", fmt.Errorf("parsing compose YAML: %w", err)
	}

	servicesRaw, ok := doc["services"]
	if !ok {
		return composeYAML, nil
	}
	services, ok := servicesRaw.(map[string]interface{})
	if !ok {
		return composeYAML, nil
	}

	// Build slot lookup
	slotMap := make(map[string]template.CodeSlot)
	for _, s := range slots {
		slotMap[s.Name] = s
	}

	// Group sources by target service
	type mount struct {
		hostPath      string
		containerPath string
	}
	svcMounts := make(map[string][]mount) // service name -> mounts

	for _, cs := range sources {
		slot, ok := slotMap[cs.Slot]
		if !ok || slot.InjectMode() != "volume" {
			continue
		}

		hostPath := filepath.Join(codeDir, appName, cs.Slot)
		if cs.Name != "" {
			hostPath = filepath.Join(hostPath, cs.Name)
		}

		containerPath := slot.Container
		if cs.Name != "" {
			containerPath = strings.ReplaceAll(containerPath, "{name}", cs.Name)
		} else {
			// Single-item slot — strip any leftover {name} placeholder
			containerPath = strings.ReplaceAll(containerPath, "{name}", "")
			containerPath = filepath.Clean(containerPath)
		}

		targetSvc := slot.Service
		if targetSvc == "" {
			targetSvc = findPrimaryService(services, appName)
		}

		svcMounts[targetSvc] = append(svcMounts[targetSvc], mount{
			hostPath:      hostPath,
			containerPath: containerPath,
		})
	}

	// Inject mounts into each service
	for svcName, mounts := range svcMounts {
		svcRaw, ok := services[svcName]
		if !ok {
			continue
		}
		svc, ok := svcRaw.(map[string]interface{})
		if !ok {
			continue
		}

		existing := getVolumes(svc)
		for _, m := range mounts {
			vol := m.hostPath + ":" + m.containerPath
			if !containsVolume(existing, vol) {
				existing = append(existing, vol)
			}
		}
		svc["volumes"] = existing
		services[svcName] = svc
	}

	doc["services"] = services
	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshalling compose YAML: %w", err)
	}
	return string(out), nil
}

// RemoveCodeVolumes removes bind mount volumes for the given code sources from the compose YAML.
// This patches the existing file in-place rather than re-rendering from template.
func RemoveCodeVolumes(composeYAML string, slots []template.CodeSlot, sources []Source, codeDir, appName string) (string, error) {
	if len(sources) == 0 {
		return composeYAML, nil
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil {
		return "", fmt.Errorf("parsing compose YAML: %w", err)
	}

	servicesRaw, ok := doc["services"]
	if !ok {
		return composeYAML, nil
	}
	services, ok := servicesRaw.(map[string]interface{})
	if !ok {
		return composeYAML, nil
	}

	// Build set of volume strings to remove
	removeSet := make(map[string]bool)
	slotMap := make(map[string]template.CodeSlot)
	for _, s := range slots {
		slotMap[s.Name] = s
	}
	for _, cs := range sources {
		slot, ok := slotMap[cs.Slot]
		if !ok || slot.InjectMode() != "volume" {
			continue
		}
		hostPath := filepath.Join(codeDir, appName, cs.Slot)
		if cs.Name != "" {
			hostPath = filepath.Join(hostPath, cs.Name)
		}
		containerPath := slot.Container
		if cs.Name != "" {
			containerPath = strings.ReplaceAll(containerPath, "{name}", cs.Name)
		} else {
			containerPath = strings.ReplaceAll(containerPath, "{name}", "")
			containerPath = filepath.Clean(containerPath)
		}
		removeSet[hostPath+":"+containerPath] = true
	}

	// Remove matching volumes from all services
	for svcName, svcRaw := range services {
		svc, ok := svcRaw.(map[string]interface{})
		if !ok {
			continue
		}
		existing := getVolumes(svc)
		if len(existing) == 0 {
			continue
		}
		var kept []string
		for _, v := range existing {
			if !removeSet[v] {
				kept = append(kept, v)
			}
		}
		if len(kept) == 0 {
			delete(svc, "volumes")
		} else {
			svc["volumes"] = kept
		}
		services[svcName] = svc
	}

	doc["services"] = services
	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshalling compose YAML: %w", err)
	}
	return string(out), nil
}

// CopyBuildSources copies code into the app dir for "build" code slots so that
// it is available inside the Docker build context. Symlinks won't work because
// Docker does not follow symlinks that point outside the build context.
// Returns the paths that were created, so they can be cleaned up after build
// with CleanBuildSources.
func CopyBuildSources(appDir string, slots []template.CodeSlot, sources []Source, codeDir, appName string, runner *executil.Runner) ([]string, error) {
	slotMap := make(map[string]template.CodeSlot)
	for _, s := range slots {
		slotMap[s.Name] = s
	}

	var created []string
	for _, cs := range sources {
		slot, ok := slotMap[cs.Slot]
		if !ok || slot.InjectMode() != "build" {
			continue
		}

		codePath := filepath.Join(codeDir, appName, cs.Slot)
		if cs.Name != "" {
			codePath = filepath.Join(codePath, cs.Name)
		}

		targetPath := filepath.Join(appDir, cs.Slot)
		if cs.Name != "" {
			targetPath = filepath.Join(appDir, cs.Slot, cs.Name)
		}

		if err := syncDir(runner, codePath, targetPath, false); err != nil {
			return created, fmt.Errorf("copying build source %s to build context: %w", cs.Slot, err)
		}
		created = append(created, targetPath)
	}
	return created, nil
}

// CleanBuildSources removes previously copied build source directories from the app dir.
func CleanBuildSources(paths []string) {
	for _, p := range paths {
		_ = os.RemoveAll(p)
	}
}

// findPrimaryService finds the primary service in a compose file, matching by
// container_name or first service alphabetically.
func findPrimaryService(services map[string]interface{}, appName string) string {
	// Check for container_name match first
	for name, svcRaw := range services {
		svc, ok := svcRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if cn, ok := svc["container_name"].(string); ok && cn == appName {
			return name
		}
	}

	// Fall back to first service alphabetically for determinism
	var names []string
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

// getVolumes extracts the volumes list from a service as a string slice.
func getVolumes(svc map[string]interface{}) []string {
	raw, ok := svc["volumes"]
	if !ok {
		return nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// containsVolume checks if a volume string already exists in the list.
func containsVolume(volumes []string, vol string) bool {
	for _, v := range volumes {
		if v == vol {
			return true
		}
	}
	return false
}
