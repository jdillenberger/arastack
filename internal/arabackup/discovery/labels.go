package discovery

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// composeFile represents a minimal docker-compose.yml structure.
type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

// composeService represents a service in docker-compose.yml.
type composeService struct {
	Labels interface{} `yaml:"labels"`
}

// parseComposeLabels reads a docker-compose.yml file and extracts arabackup.* labels.
func parseComposeLabels(composePath string) ([]ServiceBackupConfig, error) {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", composePath, err)
	}

	var compose composeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", composePath, err)
	}

	var services []ServiceBackupConfig
	for name, svc := range compose.Services {
		labels := extractLabels(svc.Labels)
		if len(labels) == 0 {
			continue
		}

		backupLabels := parseBackupLabels(labels)
		if !backupLabels.Enable {
			continue
		}

		services = append(services, ServiceBackupConfig{
			ServiceName: name,
			Labels:      backupLabels,
		})
	}

	return services, nil
}

// extractLabels normalizes Docker Compose labels (can be map or list format).
func extractLabels(raw interface{}) map[string]string {
	if raw == nil {
		return nil
	}

	labels := make(map[string]string)

	switch v := raw.(type) {
	case map[string]interface{}:
		for key, val := range v {
			if !strings.HasPrefix(key, "arabackup.") {
				continue
			}
			labels[key] = fmt.Sprintf("%v", val)
		}
	case []interface{}:
		// List format: "key=value"
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			parts := strings.SplitN(s, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			if !strings.HasPrefix(key, "arabackup.") {
				continue
			}
			labels[key] = strings.TrimSpace(parts[1])
		}
	}

	return labels
}

// parseBackupLabels converts raw arabackup.* labels into a BackupLabels struct.
func parseBackupLabels(labels map[string]string) BackupLabels {
	bl := BackupLabels{}

	if v, ok := labels["arabackup.enable"]; ok {
		bl.Enable = strings.EqualFold(v, "true")
	}

	// Borg settings
	if v, ok := labels["arabackup.borg.paths"]; ok {
		bl.BorgPaths = v
	}

	// Dump settings
	if v, ok := labels["arabackup.dump.driver"]; ok {
		bl.DumpDriver = v
	}
	if v, ok := labels["arabackup.dump.user"]; ok {
		bl.DumpUser = v
	}
	if v, ok := labels["arabackup.dump.password-env"]; ok {
		bl.DumpPasswordEnv = v
	}
	if v, ok := labels["arabackup.dump.database"]; ok {
		bl.DumpDatabase = v
	}
	if v, ok := labels["arabackup.dump.command"]; ok {
		bl.DumpCommand = v
	}
	if v, ok := labels["arabackup.dump.restore-command"]; ok {
		bl.DumpRestoreCommand = v
	}
	if v, ok := labels["arabackup.dump.file-ext"]; ok {
		bl.DumpFileExt = v
	}

	// Retention overrides
	if v, ok := labels["arabackup.retention.keep-daily"]; ok {
		bl.RetentionKeepDaily = v
	}
	if v, ok := labels["arabackup.retention.keep-weekly"]; ok {
		bl.RetentionKeepWeekly = v
	}
	if v, ok := labels["arabackup.retention.keep-monthly"]; ok {
		bl.RetentionKeepMonthly = v
	}

	return bl
}
