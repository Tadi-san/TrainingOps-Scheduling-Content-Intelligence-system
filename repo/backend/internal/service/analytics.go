package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"trainingops/internal/model"
)

type AnalyticsEngine struct{}

func NewAnalyticsEngine() *AnalyticsEngine {
	return &AnalyticsEngine{}
}

func (e *AnalyticsEngine) ComputeCohorts(events []model.ObservabilityEvent, now time.Time) []model.CohortProfile {
	windows := []struct {
		label string
		dur   time.Duration
	}{
		{label: "7d", dur: 7 * 24 * time.Hour},
		{label: "30d", dur: 30 * 24 * time.Hour},
		{label: "90d", dur: 90 * 24 * time.Hour},
	}
	out := make([]model.CohortProfile, 0, len(windows))
	for _, window := range windows {
		total := 0
		success := 0
		failureCategories := map[string]int{}
		for _, event := range events {
			if now.Sub(event.CreatedAt) > window.dur {
				continue
			}
			if event.Type == "booking" || event.Type == "scrape" {
				total++
			}
			if strings.Contains(strings.ToLower(event.Level), "error") || strings.Contains(strings.ToLower(event.Detail), "fail") {
				failureCategories[event.Type]++
			} else {
				success++
			}
		}
		topCategory := ""
		topCount := 0
		for kind, count := range failureCategories {
			if count > topCount {
				topCategory = kind
				topCount = count
			}
		}
		successRate := 0.0
		failureRate := 0.0
		if total > 0 {
			successRate = float64(success) / float64(total) * 100
			failureRate = 100 - successRate
		}
		out = append(out, model.CohortProfile{
			Window:      window.label,
			Total:       total,
			SuccessRate: successRate,
			FailureRate: failureRate,
			TopCategory: topCategory,
			UpdatedAt:   now,
		})
	}
	return out
}

func (e *AnalyticsEngine) DetectAnomalies(events []model.ObservabilityEvent, now time.Time) []model.Anomaly {
	var bookingFails, scrapingFails int
	for _, event := range events {
		if strings.EqualFold(event.Type, "booking") && strings.Contains(strings.ToLower(event.Detail), "failed") {
			bookingFails++
		}
		if strings.EqualFold(event.Type, "scrape") && strings.Contains(strings.ToLower(event.Detail), "error") {
			scrapingFails++
		}
	}
	var anomalies []model.Anomaly
	if bookingFails >= 3 {
		anomalies = append(anomalies, model.Anomaly{
			Kind:       "failed_bookings_spike",
			Severity:   "high",
			Message:    fmt.Sprintf("failed bookings spiked to %d events", bookingFails),
			Count:      bookingFails,
			Window:     "24h",
			DetectedAt: now,
		})
	}
	if scrapingFails >= 3 {
		anomalies = append(anomalies, model.Anomaly{
			Kind:       "scraping_errors_spike",
			Severity:   "medium",
			Message:    fmt.Sprintf("scraping errors spiked to %d events", scrapingFails),
			Count:      scrapingFails,
			Window:     "24h",
			DetectedAt: now,
		})
	}
	sort.Slice(anomalies, func(i, j int) bool { return anomalies[i].Kind < anomalies[j].Kind })
	return anomalies
}

func (e *AnalyticsEngine) BuildFeatureStore(cohorts []model.CohortProfile) map[string]float64 {
	features := map[string]float64{}
	for _, cohort := range cohorts {
		features["success_rate_"+cohort.Window] = cohort.SuccessRate
		features["failure_rate_"+cohort.Window] = cohort.FailureRate
	}
	return features
}
