package scraper_test

import (
	"testing"

	"github.com/azzimoda/raspishika-go/internal/bots/main/utils"
	"github.com/azzimoda/raspishika-go/internal/services"
	"github.com/azzimoda/raspishika-go/internal/services/scraper"
)

var debugConfigFile string
var templateFile string

func init() {
	utils.InitPaths()
}

func TestScrapeScheduleWithBrowser(t *testing.T) {
	testsDir := t.TempDir()
	utils.InitConfig(t, testsDir)

	srvs, err := services.NewServices()
	if err != nil {
		t.Fatal(err)
	}

	// Prepare test data.
	if _, err := scraper.FetchGroups(srvs.Repo, srvs.Browser, srvs.Cache); err != nil {
		t.Fatalf("Failed to fetch groups: %v", err)
	}

	departmentIDs, err := srvs.Repo.GetDepartmentIDs()
	if err != nil {
		t.Fatalf("Failed to get department IDs: %v", err)
	}

	teachers, err := scraper.FetchTeachers(srvs.Repo, srvs.Browser)
	if err != nil {
		t.Fatalf("Failed to fetch teachers: %v", err)
	}

	teacher := teachers[42]
	t.Log(teacher)
	teacherScheduleConfig := scraper.TeacherScheduleConfig(&teacher)
	teacherScheduleURL := scraper.ScheduleURL(teacherScheduleConfig, departmentIDs)

	// Test.
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		url     string
		config  scraper.ScheduleConfig
		want    *scraper.RawSchedule
		wantErr bool
	}{
		{"scraper schedule of teacher", teacherScheduleURL, teacherScheduleConfig, nil, false},
		{"scraper schedule of teacher with invalid URL", "https://example.com", scraper.ScheduleConfig{}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := scraper.ScrapeScheduleWithBrowser(srvs.Browser, tt.url, tt.config)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("ScrapeScheduleWithBrowser() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("ScrapeScheduleWithBrowser() succeeded unexpectedly")
			}
			if got == nil {
				t.Errorf("ScrapeScheduleWithBrowser() = %v, want %v", got, tt.want)
			}
		})
	}

	utils.Cleanup(t, testsDir)
}
