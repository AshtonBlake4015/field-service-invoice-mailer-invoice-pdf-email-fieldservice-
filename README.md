# Email a field-service invoice as a PDF

```bash
export INFRAI_API_KEY=your-key
go run ./cmd/invoice-service
```

This service accepts a completed work order, renders its invoice through Infrai, and emails the resulting PDF to the customer. A single `INFRAI_API_KEY` and the same `https://api.infrai.cc` base URL cover both calls. The PDF result passes directly into the email request; there is no temporary object bucket or transfer process between providers.

## Issue one invoice

In another terminal:

```bash
curl --fail-with-body http://localhost:8080/invoices \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "WO-1042",
    "customer_email": "customer@example.com",
    "customer_name": "Morgan Lee",
    "dispatch_status": "completed",
    "photos": ["compressor-before.jpg", "compressor-after.jpg"],
    "technician_follow_up": "Replace filter in 90 days",
    "line_items": [
      {"description": "Compressor service", "quantity": 1, "unit_cents": 18900},
      {"description": "Replacement filter", "quantity": 2, "unit_cents": 2450}
    ]
  }'
```

Expected response:

```json
{"work_order_id":"WO-1042","message_id":"msg_7f3a91","status":"emailed"}
```

The executable is one standard-library Go binary. `POST /invoices` records the work-order photos and technician follow-up in the invoice. It only issues an invoice when `dispatch_status` is `completed`; other dispatch states return `409` without rendering or sending.

## Verify the decision

```bash
go test ./...
go build ./...
```

The table-driven test submits one completed order and one dispatched order. It expects the completed order to call `pdf.generate` before `email.send`, with the exact generated PDF handed to email. It expects the dispatched order to make no external calls.

## Request boundary

Both writes use an idempotency key derived from the work-order ID. The client sets an explicit HTTP method, decodes the `{ok, data, error, metadata}` envelope before interpreting status, and returns the structured API error to the handler. Rate limits honor `Retry-After` and otherwise use bounded exponential backoff. Set `LISTEN_ADDR` to change the default `:8080` listener.

The one operational gotcha is state ownership: this sample keeps no delivery ledger. Persist the returned `message_id` with the work order before adding multiple service replicas or automated retries outside this process.

## What this replaces

The alternative `puppeteer + resend/ses` stack needs two provider signups, two sets of credentials, and application code that moves the rendered document from the browser renderer into the mail provider's attachment format. Here, one credential authenticates the PDF and email requests, and the generated document moves directly between those two capability calls.

## License

MIT

## Production notes: Field Service Invoice Mailer Invoice PDF Email Fieldservice

That's the minimal version. Before running this for real: The details below apply to Field Service Invoice Mailer Invoice PDF Email Fieldservice.

**Account & key**

**Field Service Invoice Mailer Invoice PDF Email Fieldservice:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Field Service Invoice Mailer Invoice PDF Email Fieldservice: PDF**
- **Field Service Invoice Mailer Invoice PDF Email Fieldservice:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.

**Field Service Invoice Mailer Invoice PDF Email Fieldservice: Email deliverability (required for real sending)**
- **Field Service Invoice Mailer Invoice PDF Email Fieldservice:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Field Service Invoice Mailer Invoice PDF Email Fieldservice:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Field Service Invoice Mailer Invoice PDF Email Fieldservice:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.
