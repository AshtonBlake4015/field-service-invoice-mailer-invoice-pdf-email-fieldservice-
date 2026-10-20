package fieldservice

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type recordingGateway struct {
	calls   []string
	pdfSeen string
}

func (g *recordingGateway) GeneratePDF(_ context.Context, document, _ string) (string, error) {
	g.calls = append(g.calls, "pdf.generate")
	if !strings.Contains(document, "Technician follow-up: Replace filter in 90 days") || !strings.Contains(document, "compressor-before.jpg") {
		return "", errors.New("service record missing from invoice")
	}
	return "https://documents.example/invoice.pdf", nil
}

func (g *recordingGateway) SendInvoice(_ context.Context, _, _, pdf, _ string) (string, error) {
	g.calls = append(g.calls, "email.send")
	g.pdfSeen = pdf
	return "msg_123", nil
}

func TestIssueDecision(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		wantError error
		wantCalls []string
	}{
		{name: "completed order is rendered then emailed", status: "completed", wantCalls: []string{"pdf.generate", "email.send"}},
		{name: "dispatched order is not billable", status: "dispatched", wantError: ErrWorkOrderNotComplete},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := &recordingGateway{}
			issuer := NewIssuer(gateway)
			result, err := issuer.Issue(context.Background(), WorkOrder{
				ID: "WO-1042", CustomerEmail: "customer@example.com", CustomerName: "Morgan Lee",
				DispatchStatus: tt.status, Photos: []string{"compressor-before.jpg"},
				TechnicianFollowUp: "Replace filter in 90 days",
				LineItems:          []LineItem{{Description: "Compressor service", Quantity: 1, UnitCents: 18900}},
			})
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("Issue() error = %v, want %v", err, tt.wantError)
			}
			if strings.Join(gateway.calls, ",") != strings.Join(tt.wantCalls, ",") {
				t.Fatalf("calls = %v, want %v", gateway.calls, tt.wantCalls)
			}
			if tt.wantError == nil {
				if result.Status != "emailed" || result.MessageID != "msg_123" {
					t.Fatalf("result = %+v", result)
				}
				if gateway.pdfSeen != "https://documents.example/invoice.pdf" {
					t.Fatalf("email received PDF %q", gateway.pdfSeen)
				}
			}
		})
	}
}
