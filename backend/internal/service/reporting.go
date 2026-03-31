package service

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"trainingops/internal/model"
)

type ReportFile struct {
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Path      string    `json:"-"`
}

type ReportingService struct {
	OutputPath string
}

func NewReportingService(outputPath string) *ReportingService {
	return &ReportingService{OutputPath: outputPath}
}

func (r *ReportingService) BookingsCSV(bookings []model.Booking) model.Report {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"id", "tenant_id", "room_id", "instructor_id", "title", "start_at", "end_at", "status"})
	for _, booking := range bookings {
		_ = w.Write([]string{
			booking.ID,
			booking.TenantID,
			booking.RoomID,
			booking.InstructorID,
			booking.Title,
			booking.StartAt.Format(time.RFC3339),
			booking.EndAt.Format(time.RFC3339),
			string(booking.Status),
		})
	}
	w.Flush()
	return model.Report{
		Filename: "bookings.csv",
		MimeType: "text/csv",
		Body:     buf.Bytes(),
	}
}

func (r *ReportingService) CompliancePDF(title, watermark string, lines []string) model.Report {
	content := buildMinimalPDF(title, watermark, lines)
	return model.Report{
		Filename: "compliance-report.pdf",
		MimeType: "application/pdf",
		Body:     content,
	}
}

func (r *ReportingService) WriteReport(report model.Report) (ReportFile, error) {
	if r.OutputPath == "" {
		r.OutputPath = "./reports"
	}
	if err := os.MkdirAll(r.OutputPath, 0o755); err != nil {
		return ReportFile{}, err
	}
	filename := fmt.Sprintf("%d_%s", time.Now().UTC().UnixNano(), report.Filename)
	path := filepath.Join(r.OutputPath, filename)
	if err := os.WriteFile(path, report.Body, 0o644); err != nil {
		return ReportFile{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return ReportFile{}, err
	}
	return ReportFile{
		Filename:  filename,
		Size:      info.Size(),
		CreatedAt: info.ModTime().UTC(),
		Path:      path,
	}, nil
}

func (r *ReportingService) Resolve(filename string) (string, error) {
	if filename == "" {
		return "", os.ErrNotExist
	}
	path := filepath.Join(r.OutputPath, filepath.Base(filename))
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

func buildMinimalPDF(title, watermark string, lines []string) []byte {
	var textLines []string
	textLines = append(textLines, title)
	textLines = append(textLines, "WATERMARK: "+watermark)
	textLines = append(textLines, lines...)
	escaped := strings.ReplaceAll(strings.Join(textLines, "\n"), "(", "\\(")
	escaped = strings.ReplaceAll(escaped, ")", "\\)")
	stream := fmt.Sprintf("BT /F1 18 Tf 72 760 Td (%s) Tj ET", escaped)
	return []byte("%PDF-1.4\n1 0 obj<<>>endobj\n2 0 obj<< /Type /Catalog /Pages 3 0 R >>endobj\n3 0 obj<< /Type /Pages /Count 1 /Kids [4 0 R] >>endobj\n4 0 obj<< /Type /Page /Parent 3 0 R /MediaBox [0 0 612 792] /Contents 5 0 R /Resources << /Font << /F1 6 0 R >> >> >>endobj\n5 0 obj<< /Length " + fmt.Sprint(len(stream)) + " >>stream\n" + stream + "\nendstream endobj\n6 0 obj<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>endobj\nxref\n0 7\n0000000000 65535 f \ntrailer<< /Root 2 0 R /Size 7 >>\nstartxref\n0\n%%EOF")
}
