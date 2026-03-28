package scraper_test

import (
	"testing"

	"github.com/azzimoda/raspishika-go/internal/model"
	"github.com/azzimoda/raspishika-go/internal/service"
	"github.com/azzimoda/raspishika-go/internal/service/schedule/scraper"
	"github.com/azzimoda/raspishika-go/internal/testutil"
	"github.com/azzimoda/raspishika-go/pkg/database"
)

func TestScrapeScheduleWithBrowser(t *testing.T) {
	testsDir := t.TempDir()
	testutil.InitConfig(t, testsDir)
	defer testutil.Cleanup(t, testsDir)

	db, err := database.New()
	if err != nil {
		t.Fatal(err)
	}

	srvs, err := service.New(db, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Prepare test data.
	if _, err := scraper.FetchGroups(srvs.Group, srvs.Browser); err != nil {
		t.Fatalf("Failed to fetch groups: %v", err)
	}

	departmentIDs, err := srvs.Group.DepartmentIDs()
	if err != nil {
		t.Fatalf("Failed to get department IDs: %v", err)
	}

	teachers, err := scraper.FetchTeachers(srvs.Group, srvs.Browser)
	if err != nil {
		t.Fatalf("Failed to fetch teachers: %v", err)
	}

	teacher := teachers[42]
	t.Log(teacher)
	teacherScheduleConfig := model.TeacherScheduleConfig(&teacher, false)
	teacherScheduleURL := scraper.ScheduleURL(teacherScheduleConfig, departmentIDs)

	// Test.
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		url     model.URL
		config  model.ScheduleConfig
		want    *model.RawSchedule
		wantErr bool
	}{
		{"scraper schedule of teacher", teacherScheduleURL, teacherScheduleConfig, nil, false},
		{"scraper schedule of teacher with invalid URL", "https://example.com", model.ScheduleConfig{}, nil, true},
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
