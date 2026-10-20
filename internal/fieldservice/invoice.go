package fieldservice

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
)

var ErrWorkOrderNotComplete = errors.New("work order must be completed before invoicing")

type LineItem struct {
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	UnitCents   int    `json:"unit_cents"`
}

type WorkOrder struct {
	ID                 string     `json:"id"`
	CustomerEmail      string     `json:"customer_email"`
	CustomerName       string     `json:"customer_name"`
	DispatchStatus     string     `json:"dispatch_status"`
	Photos             []string   `json:"photos"`
	TechnicianFollowUp string     `json:"technician_follow_up"`
	LineItems          []LineItem `json:"line_items"`
}

type Delivery struct {
	WorkOrderID string `json:"work_order_id"`
	MessageID   string `json:"message_id"`
	Status      string `json:"status"`
}

type Gateway interface {
	GeneratePDF(context.Context, string, string) (string, error)
	SendInvoice(context.Context, string, string, string, string) (string, error)
}

type Issuer struct{ gateway Gateway }

func NewIssuer(gateway Gateway) *Issuer { return &Issuer{gateway: gateway} }

func (i *Issuer) Issue(ctx context.Context, order WorkOrder) (Delivery, error) {
	if err := validate(order); err != nil {
		return Delivery{}, err
	}
	if order.DispatchStatus != "completed" {
		return Delivery{}, ErrWorkOrderNotComplete
	}

	requestID := "work-order-" + order.ID
	pdf, err := i.gateway.GeneratePDF(ctx, invoiceHTML(order), requestID+"-pdf")
	if err != nil {
		return Delivery{}, err
	}
	messageID, err := i.gateway.SendInvoice(ctx, order.CustomerEmail, "Invoice for work order "+order.ID, pdf, requestID+"-email")
	if err != nil {
		return Delivery{}, err
	}
	return Delivery{WorkOrderID: order.ID, MessageID: messageID, Status: "emailed"}, nil
}

func validate(order WorkOrder) error {
	if strings.TrimSpace(order.ID) == "" || strings.TrimSpace(order.CustomerEmail) == "" {
		return errors.New("id and customer_email are required")
	}
	if len(order.LineItems) == 0 {
		return errors.New("at least one line item is required")
	}
	for _, item := range order.LineItems {
		if item.Quantity <= 0 || item.UnitCents < 0 {
			return errors.New("line item quantity and unit_cents are invalid")
		}
	}
	return nil
}

func invoiceHTML(order WorkOrder) string {
	var rows strings.Builder
	total := 0
	for _, item := range order.LineItems {
		amount := item.Quantity * item.UnitCents
		total += amount
		fmt.Fprintf(&rows, "<tr><td>%s</td><td>%d</td><td>$%.2f</td><td>$%.2f</td></tr>",
			html.EscapeString(item.Description), item.Quantity, float64(item.UnitCents)/100, float64(amount)/100)
	}
	photos := "None recorded"
	if len(order.Photos) > 0 {
		photos = html.EscapeString(strings.Join(order.Photos, ", "))
	}
	return fmt.Sprintf(`<!doctype html><html><head><style>body{font-family:sans-serif;color:#1f2937}table{width:100%%;border-collapse:collapse}th,td{padding:8px;border-bottom:1px solid #ddd;text-align:left}.total{text-align:right;font-size:18px}</style></head><body><h1>Field service invoice</h1><p>Work order: %s</p><p>Customer: %s</p><p>Dispatch status: %s</p><table><thead><tr><th>Work</th><th>Qty</th><th>Unit</th><th>Amount</th></tr></thead><tbody>%s</tbody></table><p class="total">Total: $%.2f</p><h2>Service record</h2><p>Photos: %s</p><p>Technician follow-up: %s</p></body></html>`,
		html.EscapeString(order.ID), html.EscapeString(order.CustomerName), html.EscapeString(order.DispatchStatus), rows.String(), float64(total)/100, photos, html.EscapeString(order.TechnicianFollowUp))
}
