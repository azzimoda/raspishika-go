package scraper_test

import (
	"testing"

	tests "github.com/azzimoda/raspishika-go/internal/bots/main/tests"
	"github.com/azzimoda/raspishika-go/internal/models"
	"github.com/azzimoda/raspishika-go/internal/services"
	"github.com/azzimoda/raspishika-go/internal/services/schedule/scraper"
)

var debugConfigFile string
var templateFile string

func TestScrapeScheduleWithBrowser(t *testing.T) {
	testsDir := t.TempDir()
	tests.InitConfig(t, testsDir)
	defer tests.Cleanup(t, testsDir)

	srvs, err := services.New()
	if err != nil {
		t.Fatal(err)
	}

	// Prepare test data.
	if _, err := scraper.FetchGroups(srvs.Repo, srvs.Browser); err != nil {
		t.Fatalf("Failed to fetch groups: %v", err)
	}

	departmentIDs, err := models.GetDepartmentIDs(srvs.Repo.DB)
	if err != nil {
		t.Fatalf("Failed to get department IDs: %v", err)
	}

	teachers, err := scraper.FetchTeachers(srvs.Repo, srvs.Browser)
	if err != nil {
		t.Fatalf("Failed to fetch teachers: %v", err)
	}

	teacher := teachers[42]
	t.Log(teacher)
	teacherScheduleConfig := models.TeacherScheduleConfig(&teacher)
	teacherScheduleURL := scraper.ScheduleURL(teacherScheduleConfig, departmentIDs)

	// Test.
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		url     models.URL
		config  models.ScheduleConfig
		want    *models.RawSchedule
		wantErr bool
	}{
		{"scraper schedule of teacher", teacherScheduleURL, teacherScheduleConfig, nil, false},
		{"scraper schedule of teacher with invalid URL", "https://example.com", models.ScheduleConfig{}, nil, true},
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
}
