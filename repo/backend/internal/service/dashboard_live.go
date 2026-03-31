package service

import (
	"context"
	"strconv"
	"time"

	"trainingops/internal/model"
	"trainingops/internal/repository"
)

type DashboardService struct {
	Bookings repository.BookingStore
}

func NewDashboardService(bookings repository.BookingStore) *DashboardService {
	return &DashboardService{Bookings: bookings}
}

func (s *DashboardService) Build(ctx context.Context, tenantID, role string, now time.Time) (model.DashboardData, error) {
	bookings, err := s.Bookings.ListBookings(ctx, tenantID)
	if err != nil {
		return model.DashboardData{}, err
	}

	total := len(bookings)
	held := 0
	confirmed := 0
	checkedIn := 0
	cancelled := 0
	totalStudyMinutes := 0
	instructorSet := map[string]bool{}
	for _, booking := range bookings {
		durationMinutes := int(booking.EndAt.Sub(booking.StartAt).Minutes())
		if durationMinutes < 0 {
			durationMinutes = 0
		}
		switch booking.Status {
		case model.BookingStatusHeld:
			held++
		case model.BookingStatusConfirmed:
			confirmed++
			totalStudyMinutes += durationMinutes
		case model.BookingStatusCheckedIn:
			checkedIn++
			totalStudyMinutes += durationMinutes
		case model.BookingStatusExpired, model.BookingStatusCancelled:
			cancelled++
		}
		if booking.InstructorID != "" {
			instructorSet[booking.InstructorID] = true
		}
	}
	enrollmentGrowth := confirmed + checkedIn - cancelled
	repeatAttendance := percent(checkedIn, confirmed+checkedIn)
	contentConversion := percent(checkedIn, held+confirmed+checkedIn)

	heatmap := buildHeatmap(bookings)
	calendar := make([]model.CalendarSession, 0, len(bookings))
	for _, booking := range bookings {
		calendar = append(calendar, model.CalendarSession{
			ID:       booking.ID,
			Title:    booking.Title,
			StartsAt: booking.StartAt,
			EndsAt:   booking.EndAt,
			Room:     booking.RoomID,
			Owner:    booking.InstructorID,
			Status:   string(booking.Status),
		})
	}
	roleLabel := role
	if roleLabel == "" {
		roleLabel = "learner"
	}

	data := model.DashboardData{
		Role:            roleLabel,
		Title:           titleForRole(roleLabel),
		Subtitle:        subtitleForRole(roleLabel),
		KPIs:            []model.KPI{},
		Heatmap:         heatmap,
		Calendar:        calendar,
		TaskOrdering:    defaultTaskOrder(roleLabel),
		PreviewDocument: "trainingops-overview.pdf",
		PreviewImage:    "trainingops-dashboard.png",
	}

	data.KPIs = append(data.KPIs,
		model.KPI{Label: "Enrollment growth", Value: strconv.Itoa(enrollmentGrowth), Delta: "net active enrollments"},
		model.KPI{Label: "Repeat attendance", Value: strconv.Itoa(repeatAttendance) + "%", Delta: "checked-in vs active"},
		model.KPI{Label: "Study time logged", Value: strconv.Itoa(totalStudyMinutes) + "m", Delta: "scheduled learning minutes"},
		model.KPI{Label: "Content conversion", Value: strconv.Itoa(contentConversion) + "%", Delta: "holds to completion"},
		model.KPI{Label: "Community activity", Value: strconv.Itoa(len(instructorSet)) + " facilitators", Delta: strconv.Itoa(total) + " sessions"},
	)
	if roleLabel == "coordinator" {
		end := now.Add(5 * time.Minute)
		data.CountdownEnd = &end
	}

	return data, nil
}

func buildHeatmap(bookings []model.Booking) []model.HeatmapCell {
	type key struct {
		day  string
		hour int
	}
	loadMap := map[key]int{}
	for _, booking := range bookings {
		day := booking.StartAt.UTC().Weekday().String()[:3]
		hour := booking.StartAt.UTC().Hour()
		slot := key{day: day, hour: hour}
		loadMap[slot] += 20
	}

	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	cells := make([]model.HeatmapCell, 0, len(days)*12)
	for _, day := range days {
		for hour := 8; hour <= 19; hour++ {
			load := loadMap[key{day: day, hour: hour}]
			if load > 100 {
				load = 100
			}
			state := "low"
			switch {
			case load >= 70:
				state = "high"
			case load >= 40:
				state = "medium"
			}
			cells = append(cells, model.HeatmapCell{Day: day, Hour: hour, Load: load, State: state})
		}
	}
	return cells
}

func titleForRole(role string) string {
	switch role {
	case "admin":
		return "Admin command center"
	case "coordinator":
		return "Scheduling control room"
	case "instructor":
		return "Instructor workspace"
	default:
		return "Learner workspace"
	}
}

func subtitleForRole(role string) string {
	switch role {
	case "admin":
		return "Live organizational signals from bookings and operations."
	case "coordinator":
		return "Live scheduling, hold windows, and booking health."
	case "instructor":
		return "Session delivery and workload visibility."
	default:
		return "Upcoming sessions and learning progress."
	}
}

func defaultTaskOrder(role string) []string {
	switch role {
	case "admin":
		return []string{"Review audit stream", "Inspect failures", "Export report", "Review tenants"}
	case "coordinator":
		return []string{"Review new holds", "Resolve conflicts", "Confirm sessions", "Publish schedule"}
	case "instructor":
		return []string{"Check class plan", "Verify materials", "Run session", "Capture attendance"}
	default:
		return []string{"Open materials", "Join class", "Track progress", "Review feedback"}
	}
}

func percent(part, whole int) int {
	if whole <= 0 {
		return 0
	}
	return int(float64(part) / float64(whole) * 100)
}
