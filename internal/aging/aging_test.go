package aging

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func date(text string) time.Time {
	parsed, err := time.Parse("2006-01-02", text)
	if err != nil {
		panic(err)
	}
	return parsed
}

var asOf = date("2026-10-31")

func dueAgo(days int) time.Time { return asOf.AddDate(0, 0, -days) }

func TestBucketBoundaries(t *testing.T) {
	cases := map[int]string{-5: "not due", 0: "not due", 1: "1-30", 30: "1-30", 31: "31-60", 60: "31-60", 61: "61-90", 90: "61-90", 91: "90+", 500: "90+"}
	for overdue, want := range cases {
		report := Build([]Invoice{{Customer: "A", Due: dueAgo(overdue), Open: 100}}, asOf)
		if report.Buckets[want] != "1.00" {
			t.Errorf("%d days: got %v, want %s", overdue, report.Buckets, want)
		}
	}
}

// Worked by hand: 100.00 due today isn't overdue, 300.00 is 10 days late.
// That's 400.00 open, 300.00 overdue, and 75%.
func TestTotalsByHand(t *testing.T) {
	report := Build([]Invoice{{Customer: "A", Due: asOf, Open: 10000}, {Customer: "A", Due: dueAgo(10), Open: 30000}}, asOf)
	if report.TotalOpen != "400.00" || report.TotalOverdue != "300.00" || report.OverduePct != 75 {
		t.Errorf("got %+v", report)
	}
	line := report.Customers[0]
	if line.Open != "400.00" || line.Overdue != "300.00" || line.OldestDays != 10 || line.Invoices[0].DaysOverdue != 10 {
		t.Errorf("got %+v", line)
	}
}

func TestPaidInvoicesDontAppearButInformLateness(t *testing.T) {
	report := Build([]Invoice{
		{Customer: "A", Due: date("2026-03-31"), PaidDate: date("2026-04-20")},
		{Customer: "A", Due: date("2026-05-01"), PaidDate: date("2026-05-31")},
		{Customer: "A", Due: dueAgo(5), Open: 100},
	}, asOf)
	line := report.Customers[0]
	if len(line.Invoices) != 1 || line.AvgDaysLate == nil || *line.AvgDaysLate != 25 {
		t.Errorf("got %+v", line)
	}
	report = Build([]Invoice{{Customer: "B", Due: dueAgo(5), PaidDate: dueAgo(1)}}, asOf)
	if len(report.Customers) != 0 {
		t.Errorf("fully paid customer listed: %+v", report.Customers)
	}
}

func TestCreditsStaySeparate(t *testing.T) {
	report := Build([]Invoice{{Number: "CR", Customer: "A", Due: dueAgo(5), Open: -7500}}, asOf)
	if report.TotalOpen != "0.00" || len(report.Customers) != 0 || len(report.Credits) != 1 || report.Credits[0].Open != "-75.00" {
		t.Errorf("got %+v", report)
	}
}

func TestPriorityWeighsAmountAndAge(t *testing.T) {
	report := Build([]Invoice{
		{Customer: "large but recent", Due: dueAgo(2), Open: 100000},
		{Customer: "old", Due: dueAgo(120), Open: 50000},
		{Customer: "not due", Due: asOf.AddDate(0, 0, 10), Open: 900000},
	}, asOf)
	var names []string
	for _, line := range report.Customers {
		names = append(names, line.Customer)
	}
	if !slices.Equal(names, []string{"old", "large but recent", "not due"}) {
		t.Errorf("got %v", names)
	}
}

func TestWarnings(t *testing.T) {
	report := Build([]Invoice{{Number: "1", Customer: "A", Due: asOf, Open: 1, Currency: "USD"}, {Number: "1", Customer: "A", Due: asOf, Open: 1, Currency: "EUR"}}, asOf)
	joined := strings.Join(report.Warnings, " ")
	if !strings.Contains(joined, "mixes currencies (EUR, USD)") || !strings.Contains(joined, "Invoice 1 appears 2 times") {
		t.Errorf("got %v", report.Warnings)
	}
}

func TestCustomersMatchIgnoringCase(t *testing.T) {
	report := Build([]Invoice{{Customer: "Acme  Ltd", Due: asOf, Open: 1}, {Customer: "acme ltd", Due: asOf, Open: 1}}, asOf)
	if len(report.Customers) != 1 {
		t.Errorf("got %+v", report.Customers)
	}
}

func cents(t *testing.T, text string) int64 {
	t.Helper()
	value, ok := ParseAmount(text)
	if !ok {
		t.Fatalf("bad amount %q", text)
	}
	return value
}

func TestInvariantsOnRandomData(t *testing.T) {
	random := rand.New(rand.NewSource(5))
	for round := 0; round < 400; round++ {
		var invoices []Invoice
		for i := 0; i < random.Intn(40); i++ {
			invoices = append(invoices, Invoice{Number: strconv.Itoa(i), Customer: fmt.Sprintf("c%d", random.Intn(6)), Due: dueAgo(random.Intn(300) - 60), Open: int64(random.Intn(100000) - 5000)})
		}
		report := Build(invoices, asOf)
		var bucketSum, customerSum int64
		for _, name := range Buckets {
			bucketSum += cents(t, report.Buckets[name])
		}
		for _, line := range report.Customers {
			customerSum += cents(t, line.Open)
			if cents(t, line.Overdue) > cents(t, line.Open) {
				t.Fatalf("overdue above open: %+v", line)
			}
		}
		if bucketSum != cents(t, report.TotalOpen) || customerSum != cents(t, report.TotalOpen) {
			t.Fatalf("totals disagree: buckets %d customers %d total %s", bucketSum, customerSum, report.TotalOpen)
		}
		if cents(t, report.TotalOverdue)+cents(t, report.Buckets["not due"]) != cents(t, report.TotalOpen) {
			t.Fatalf("overdue doesn't add up: %+v", report)
		}
	}
}

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	file, err := Load(write(t, "Report,,,\nNota;Cliente;Vencimento;Valor;Valor Pago\nA1;Ana;25/02/2026;1.000,00;250,00\nTotal;;;;\n"), nil, "", 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Invoices) != 1 || file.Invoices[0].Open != 75000 || file.DateFormat != "dmy" || !slices.Equal(file.Skipped, []int{4}) {
		t.Errorf("got %+v", file)
	}
	file, err = Load(write(t, "Invoice,Customer,Date,Balance Due,Total\nA1,Ana,2026-01-10,40.00,100.00\n"), nil, "", 15)
	if err != nil {
		t.Fatal(err)
	}
	if file.Invoices[0].Open != 4000 || file.Invoices[0].Due.Format("2006-01-02") != "2026-01-25" {
		t.Errorf("balance beats total, and terms set the due date: %+v", file.Invoices[0])
	}
	ambiguous := write(t, "Customer,Due Date,Total\nA,03/02/2026,1\nB,04/02/2026,1\n")
	if _, err := Load(ambiguous, nil, "", 30); err == nil || !strings.Contains(err.Error(), "dmy or mdy") {
		t.Errorf("got %v", err)
	}
	unknown := write(t, "Foo,Bar\n1,2\n")
	if _, err := Load(unknown, nil, "", 30); err == nil || !strings.Contains(err.Error(), "First row: Foo, Bar") {
		t.Errorf("got %v", err)
	}
	if _, err := Load(unknown, map[string]string{"customer": "Nope"}, "", 30); err == nil || !strings.Contains(err.Error(), "Columns found: Foo, Bar") {
		t.Errorf("got %v", err)
	}
	if _, err := Load("/no/such.csv", nil, "", 30); err == nil {
		t.Error("missing file should fail")
	}
	if _, err := Load(t.TempDir(), nil, "", 30); err == nil {
		t.Error("folder should fail")
	}
	noDates := write(t, "Customer,Total,Due Date\nA,1,\n")
	if file, err := Load(noDates, nil, "", 30); err != nil || len(file.Invoices) != 0 || len(file.Skipped) != 1 {
		t.Errorf("%v %+v", err, file)
	}
}
