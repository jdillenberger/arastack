package deploy

import (
	"testing"
	"time"
)

func TestRunValidator_Timezone(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
	}{
		{"UTC", false},
		{"Europe/Berlin", false},
		{"America/New_York", false},
		{"Asia/Tokyo", false},
		{"US/Eastern", false},
		{"Local", true},
		{"local", true},
		{"Invalid/Zone", true},
		{"NotATimezone", true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			err := runValidator("timezone", tt.value, "timezone")
			if (err != nil) != tt.wantErr {
				t.Errorf("runValidator(timezone, %q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestHostTimezone(t *testing.T) {
	tz := hostTimezone()
	if tz == "" {
		t.Fatal("hostTimezone() returned empty string")
	}
	if _, err := time.LoadLocation(tz); err != nil {
		t.Errorf("hostTimezone() returned invalid timezone %q: %v", tz, err)
	}
}
