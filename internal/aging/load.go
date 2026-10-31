// Package aging builds accounts receivable aging reports from invoice files.
package aging

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const maxFileBytes = 50_000_000

// Invoice is one row. Open is what's still owed, in cents, and negative for a credit.
type Invoice struct {
	Row      int
	Number   string
	Customer string
	Issued   time.Time
	Due      time.Time
	Open     int64
	PaidDate time.Time
	Currency string
}

// File is a loaded invoice export.
type File struct {
	Invoices   []Invoice
	Skipped    []int
	Columns    map[string]string
	DateFormat string
}

var fieldNames = map[string][]string{
	"number":   {"invoice", "invoice number", "invoice no", "invoice #", "invoice id", "number", "num", "doc number", "document", "fatura", "nota", "numero", "factura", "rechnung"},
	"customer": {"customer", "customer name", "client", "client name", "name", "bill to", "account", "company", "cliente", "kunde"},
	"issued":   {"invoice date", "date", "issue date", "issued", "created", "data emissao", "emissao", "fecha", "datum"},
	"due":      {"due date", "due", "payment due", "vencimento", "data vencimento", "vencimiento", "fallig"},
	"amount":   {"total", "amount", "invoice total", "invoice amount", "valor", "valor total", "importe", "betrag"},
	"paid":     {"paid", "amount paid", "payments", "paid amount", "valor pago", "pagado"},
	"balance":  {"balance", "balance due", "amount due", "open amount", "open balance", "outstanding", "saldo"},
	"paiddate": {"paid date", "date paid", "payment date", "paid on", "data pagamento", "fecha de pago"},
	"currency": {"currency", "ccy", "moeda", "moneda"},
}

var accents = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "ç", "c", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ü", "u", "ñ", "n")

func normalHeader(cell string) string {
	cell = accents.Replace(strings.ToLower(strings.TrimSpace(cell)))
	return strings.Join(strings.Fields(strings.NewReplacer("_", " ", ".", " ", ":", " ").Replace(cell)), " ")
}

func detect(header []string, overrides map[string]string) (map[string]int, error) {
	found := map[string]int{}
	normal := make([]string, len(header))
	for i, cell := range header {
		normal[i] = normalHeader(cell)
	}
	for field, names := range fieldNames {
		for _, name := range names {
			if _, set := found[field]; set {
				break
			}
			for i, cell := range normal {
				if cell == name {
					found[field] = i
					break
				}
			}
		}
	}
	for field, name := range overrides {
		if _, known := fieldNames[field]; !known {
			return nil, fmt.Errorf("unknown field %q. Use number, customer, issued, due, amount, paid, balance, paiddate or currency", field)
		}
		index := -1
		for i, cell := range normal {
			if cell == normalHeader(name) {
				index = i
			}
		}
		if index < 0 {
			return nil, fmt.Errorf("no column named %q. Columns found: %s", name, strings.Join(header, ", "))
		}
		found[field] = index
	}
	return found, nil
}

func usable(found map[string]int) bool {
	_, customer := found["customer"]
	_, amount := found["amount"]
	_, balance := found["balance"]
	_, due := found["due"]
	_, issued := found["issued"]
	return customer && (amount || balance) && (due || issued)
}

// Load reads an invoice CSV. A missing due date becomes the issue date plus
// termsDays, and a row with neither date is skipped rather than guessed.
func Load(path string, overrides map[string]string, dateFormat string, termsDays int) (*File, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("can't read %s: %w", path, errors.Unwrap(err))
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a folder. Pass a CSV file", path)
	}
	if info.Size() > maxFileBytes {
		return nil, fmt.Errorf("%s is larger than 50 MB", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("can't read %s: %w", path, err)
	}
	text := decode(data)
	reader := csv.NewReader(strings.NewReader(text))
	reader.Comma = detectDelimiter(text)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("can't parse %s as CSV: %w", path, err)
	}

	index := -1
	var columns map[string]int
	for i := 0; i < len(rows) && i < 20; i++ {
		found, err := detect(rows[i], overrides)
		if err == nil && usable(found) {
			index, columns = i, found
			break
		}
	}
	if index < 0 {
		if len(overrides) > 0 && len(rows) > 0 {
			if _, err := detect(rows[0], overrides); err != nil {
				return nil, err
			}
		}
		first := "(empty file)"
		if len(rows) > 0 {
			first = strings.Join(rows[0], ", ")
		}
		return nil, fmt.Errorf("can't find customer, amount and date columns in %s. First row: %s. Name them with a column map", path, first)
	}

	cell := func(row []string, field string) string {
		if at, ok := columns[field]; ok && at < len(row) {
			return strings.TrimSpace(row[at])
		}
		return ""
	}
	body := rows[index+1:]
	var dates []string
	for _, row := range body {
		for _, field := range []string{"issued", "due", "paiddate"} {
			if value := cell(row, field); value != "" {
				dates = append(dates, value)
			}
		}
	}
	if dateFormat == "" {
		dateFormat = DetectDateFormat(dates)
	}
	if dateFormat == "" {
		return nil, fmt.Errorf("the dates in %s could be day-first or month-first. Pass a date format of dmy or mdy", path)
	}
	file := &File{Skipped: []int{}, DateFormat: dateFormat, Columns: map[string]string{}}
	for field, at := range columns {
		file.Columns[field] = rows[index][at]
	}
	for offset, row := range body {
		number := index + offset + 2
		if strings.TrimSpace(strings.Join(row, "")) == "" {
			continue
		}
		customer := cell(row, "customer")
		issued, hasIssued := ParseDate(cell(row, "issued"), dateFormat)
		due, hasDue := ParseDate(cell(row, "due"), dateFormat)
		if !hasDue && hasIssued {
			due, hasDue = issued.AddDate(0, 0, termsDays), true
		}
		var open int64
		var hasOpen bool
		if _, hasBalance := columns["balance"]; hasBalance {
			open, hasOpen = ParseAmount(cell(row, "balance"))
		}
		if !hasOpen {
			total, hasTotal := ParseAmount(cell(row, "amount"))
			paid, _ := ParseAmount(cell(row, "paid"))
			open, hasOpen = total-paid, hasTotal
		}
		if customer == "" || !hasDue || !hasOpen {
			file.Skipped = append(file.Skipped, number)
			continue
		}
		paidDate, _ := ParseDate(cell(row, "paiddate"), dateFormat)
		file.Invoices = append(file.Invoices, Invoice{Row: number, Number: cell(row, "number"), Customer: customer, Issued: issued, Due: due, Open: open, PaidDate: paidDate, Currency: strings.ToUpper(cell(row, "currency"))})
	}
	return file, nil
}
