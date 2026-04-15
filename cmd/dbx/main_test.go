package main

import (
	"testing"
	"time"
)

func TestTimeAgo(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{name: "zero time", t: time.Time{}, want: "-"},
		{name: "30 seconds ago", t: now.Add(-30 * time.Second), want: "30s ago"},
		{name: "5 minutes ago", t: now.Add(-5 * time.Minute), want: "5m ago"},
		{name: "3 hours ago", t: now.Add(-3 * time.Hour), want: "3h ago"},
		{name: "1 day ago", t: now.Add(-24 * time.Hour), want: "1 day ago"},
		{name: "3 days ago", t: now.Add(-3 * 24 * time.Hour), want: "3 days ago"},
		{name: "1 week ago", t: now.Add(-7 * 24 * time.Hour), want: "1 week ago"},
		{name: "3 weeks ago", t: now.Add(-21 * 24 * time.Hour), want: "3 weeks ago"},
		{name: "1 month ago", t: now.Add(-30 * 24 * time.Hour), want: "1 month ago"},
		{name: "3 months ago", t: now.Add(-90 * 24 * time.Hour), want: "3 months ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := timeAgo(tt.t)
			if got != tt.want {
				t.Errorf("timeAgo() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		commit  string
		date    string
		want    string
	}{
		{
			name: "all fields", version: "1.0.0", commit: "abc123", date: "2025-01-01",
			want: "1.0.0\ncommit: abc123\nbuilt at: 2025-01-01",
		},
		{
			name: "no commit", version: "1.0.0", commit: "", date: "2025-01-01",
			want: "1.0.0\nbuilt at: 2025-01-01",
		},
		{
			name: "no date", version: "1.0.0", commit: "abc123", date: "",
			want: "1.0.0\ncommit: abc123",
		},
		{
			name: "version only", version: "dev", commit: "", date: "",
			want: "dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildVersion(tt.version, tt.commit, tt.date)
			if got != tt.want {
				t.Errorf("buildVersion(%q, %q, %q) = %q, want %q", tt.version, tt.commit, tt.date, got, tt.want)
			}
		})
	}
}
