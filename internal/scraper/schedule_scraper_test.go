package scraper_test

import (
	"testing"

	"github.com/azzimoda/raspishika-go/internal/mainbot/utils"
	"github.com/azzimoda/raspishika-go/internal/scraper"
)

var debugConfigFile string
var templateFile string

func init() {
	debugConfigFile, templateFile = utils.InitPaths()
	println(debugConfigFile, templateFile)
}

func TestScrapeScheduleWithBrowser(t *testing.T) {
	// Initialize configuration.
	testsDir, _, repo, browser, cache := utils.InitServices(t, debugConfigFile, templateFile)

	// Prepare test data.
	if _, err := scraper.FetchGroups(repo, browser, cache); err != nil {
		t.Fatalf("Failed to fetch groups: %v", err)
	}

	departmentIDs, err := repo.GetDepartmentIDs()
	if err != nil {
		t.Fatalf("Failed to get department IDs: %v", err)
	}

	teachers, err := scraper.FetchTeachers(repo, browser)
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
			got, gotErr := scraper.ScrapeScheduleWithBrowser(browser, tt.url, tt.config)
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
