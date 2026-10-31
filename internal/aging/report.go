package aging

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Bucket names, in order.
var Buckets = []string{"not due", "1-30", "31-60", "61-90", "90+"}

func bucketIndex(daysOverdue int) int {
	switch {
	case daysOverdue <= 0:
		return 0
	case daysOverdue <= 30:
		return 1
	case daysOverdue <= 60:
		return 2
	case daysOverdue <= 90:
		return 3
	}
	return 4
}

// OpenInvoice is one unpaid invoice.
type OpenInvoice struct {
	Number      string `json:"number"`
	Open        string `json:"open"`
	Due         string `json:"due"`
	DaysOverdue int    `json:"days_overdue"`
}

// CustomerLine is one customer's position.
type CustomerLine struct {
	Customer    string            `json:"customer"`
	Open        string            `json:"open"`
	Overdue     string            `json:"overdue"`
	OldestDays  int               `json:"oldest_days_overdue"`
	Buckets     map[string]string `json:"buckets"`
	AvgDaysLate *float64          `json:"usual_days_late,omitempty"`
	Invoices    []OpenInvoice     `json:"open_invoices"`
	overdue     int64
	score       float64
}

// Report is the full aging report.
type Report struct {
	AsOf         string            `json:"as_of"`
	TotalOpen    string            `json:"total_open"`
	TotalOverdue string            `json:"total_overdue"`
	OverduePct   float64           `json:"overdue_percent"`
	Buckets      map[string]string `json:"buckets"`
	Customers    []CustomerLine    `json:"customers_by_priority"`
	Credits      []OpenInvoice     `json:"credits"`
	Warnings     []string          `json:"warnings"`
}

func formatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

func customerKey(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(name)), " ")
}

func days(from, to time.Time) int {
	return int(to.Sub(from).Hours() / 24)
}

// Build ages every open invoice as of the given date. Customers are ranked by
// overdue money weighted by age, because an old debt gets harder to collect.
func Build(invoices []Invoice, asOf time.Time) Report {
	report := Report{AsOf: asOf.Format("2006-01-02"), Buckets: map[string]string{}, Customers: []CustomerLine{}, Credits: []OpenInvoice{}, Warnings: []string{}}
	type position struct {
		line    *CustomerLine
		buckets [5]int64
		open    int64
		lateSum int
		lateN   int
	}
	people := map[string]*position{}
	var totals [5]int64
	var totalOpen, totalOverdue int64
	currencies := map[string]bool{}
	numbers := map[string]int{}

	for _, invoice := range invoices {
		key := customerKey(invoice.Customer)
		person := people[key]
		if person == nil {
			person = &position{line: &CustomerLine{Customer: invoice.Customer, Invoices: []OpenInvoice{}}}
			people[key] = person
		}
		if !invoice.PaidDate.IsZero() {
			person.lateSum += days(invoice.Due, invoice.PaidDate)
			person.lateN++
		}
		if invoice.Number != "" {
			numbers[invoice.Number]++
		}
		if invoice.Currency != "" {
			currencies[invoice.Currency] = true
		}
		if invoice.Open == 0 {
			continue
		}
		overdue := days(invoice.Due, asOf)
		item := OpenInvoice{Number: invoice.Number, Open: formatCents(invoice.Open), Due: invoice.Due.Format("2006-01-02"), DaysOverdue: overdue}
		if invoice.Open < 0 {
			report.Credits = append(report.Credits, item)
			continue
		}
		if overdue < 0 {
			item.DaysOverdue = 0
		}
		at := bucketIndex(overdue)
		person.buckets[at] += invoice.Open
		totals[at] += invoice.Open
		person.open += invoice.Open
		totalOpen += invoice.Open
		person.line.Invoices = append(person.line.Invoices, item)
		if overdue > 0 {
			totalOverdue += invoice.Open
			person.line.overdue += invoice.Open
			person.line.score += float64(invoice.Open) * float64(overdue)
			if overdue > person.line.OldestDays {
				person.line.OldestDays = overdue
			}
		}
	}

	for _, person := range people {
		if person.lateN > 0 {
			average := math.Round(float64(person.lateSum)/float64(person.lateN)*10) / 10
			person.line.AvgDaysLate = &average
		}
		if person.open == 0 {
			continue
		}
		person.line.Open = formatCents(person.open)
		person.line.Overdue = formatCents(person.line.overdue)
		person.line.Buckets = map[string]string{}
		for i, name := range Buckets {
			person.line.Buckets[name] = formatCents(person.buckets[i])
		}
		sort.Slice(person.line.Invoices, func(i, j int) bool { return person.line.Invoices[i].Due < person.line.Invoices[j].Due })
		report.Customers = append(report.Customers, *person.line)
	}
	sort.Slice(report.Customers, func(i, j int) bool {
		a, b := report.Customers[i], report.Customers[j]
		if a.score != b.score {
			return a.score > b.score
		}
		return a.Customer < b.Customer
	})
	sort.Slice(report.Credits, func(i, j int) bool { return report.Credits[i].Number < report.Credits[j].Number })

	for i, name := range Buckets {
		report.Buckets[name] = formatCents(totals[i])
	}
	report.TotalOpen = formatCents(totalOpen)
	report.TotalOverdue = formatCents(totalOverdue)
	if totalOpen > 0 {
		report.OverduePct = math.Round(float64(totalOverdue)/float64(totalOpen)*1000) / 10
	}
	if len(currencies) > 1 {
		names := make([]string, 0, len(currencies))
		for name := range currencies {
			names = append(names, name)
		}
		sort.Strings(names)
		report.Warnings = append(report.Warnings, fmt.Sprintf("The file mixes currencies (%s), and totals add them without converting.", strings.Join(names, ", ")))
	}
	repeated := []string{}
	for number, count := range numbers {
		if count > 1 {
			repeated = append(repeated, number)
		}
	}
	sort.Strings(repeated)
	for _, number := range repeated {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Invoice %s appears %d times, and each row was counted.", number, numbers[number]))
	}
	return report
}
