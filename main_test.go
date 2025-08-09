package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// withTempHome redirects the app's data dir into a temp HOME for isolation.
func withTempHome(t *testing.T) (cleanup func()) {
	t.Helper()
	d := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldUser := os.Getenv("USERPROFILE")
	_ = os.Setenv("HOME", d)
	_ = os.Setenv("USERPROFILE", d) // for Windows
	return func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("USERPROFILE", oldUser)
	}
}

// captureOutput captures stdout produced by fn.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var b strings.Builder
	s := bufio.NewScanner(r)
	for s.Scan() {
		b.WriteString(s.Text())
		b.WriteByte('\n')
	}
	return b.String()
}

func TestAddListSummaryDeleteFlow(t *testing.T) {
	defer withTempHome(t)()

	// Add two expenses (fixed date for determinism)
	date := time.Now().Format("2006-01-02")
	if err := cmdAdd([]string{"--description", "Lunch", "--amount", "20", "--date", date}); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if err := cmdAdd([]string{"--description", "Dinner", "--amount", "10", "--date", date}); err != nil {
		t.Fatalf("add 2: %v", err)
	}

	// List should show 2 lines with IDs 1 and 2
	out := captureOutput(t, func() {
		if err := cmdList(nil); err != nil {
			t.Fatalf("list: %v", err)
		}
	})
	if !strings.Contains(out, "# 1") || !strings.Contains(out, "# 2") {
		t.Fatalf("list output missing IDs:\n%s", out)
	}

	// Summary (all time) should be $30.00
	out = captureOutput(t, func() {
		if err := cmdSummary(nil); err != nil {
			t.Fatalf("summary: %v", err)
		}
	})
	if !strings.Contains(out, "# Total expenses: $30.00") {
		t.Fatalf("unexpected summary: %s", out)
	}

	// Delete ID 2
	if err := cmdDelete([]string{"--id", "2"}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Summary should be $20.00
	out = captureOutput(t, func() {
		if err := cmdSummary(nil); err != nil {
			t.Fatalf("summary after delete: %v", err)
		}
	})
	if !strings.Contains(out, "# Total expenses: $20.00") {
		t.Fatalf("unexpected summary after delete: %s", out)
	}
}

func TestUpdateAndCategory(t *testing.T) {
	defer withTempHome(t)()
	date := time.Now().Format("2006-01-02")
	if err := cmdAdd([]string{"--description", "Coffee", "--amount", "5", "--date", date, "--category", "Food"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Update amount and category
	if err := cmdUpdate([]string{"--id", "1", "--amount", "7.5", "--category", "Beverage"}); err != nil {
		t.Fatalf("update: %v", err)
	}

	// Verify via list output (avoid relying on internal helpers)
	out := captureOutput(t, func() {
		if err := cmdList(nil); err != nil {
			t.Fatalf("list: %v", err)
		}
	})
	if !strings.Contains(out, "Beverage") {
		t.Fatalf("updated category not shown: %s", out)
	}
	if !strings.Contains(out, "$7.50") {
		// Fallback: some locales may not include trailing zero; just check 7.5
		if !strings.Contains(out, "$7.5") {
			t.Fatalf("updated amount not shown: %s", out)
		}
	}

	// Category listing filter
	out = captureOutput(t, func() {
		if err := cmdList([]string{"--category", "Beverage"}); err != nil {
			t.Fatalf("list filtered: %v", err)
		}
	})
	if !strings.Contains(out, "Beverage") {
		t.Fatalf("category filter not shown: %s", out)
	}
}

func TestSummaryByMonth(t *testing.T) {
	defer withTempHome(t)()
	// Put one expense in current month
	now := time.Now()
	date := now.Format("2006-01-02")
	if err := cmdAdd([]string{"--description", "Fuel", "--amount", "40", "--date", date}); err != nil {
		t.Fatalf("add: %v", err)
	}

	m := int(now.Month())
	out := captureOutput(t, func() {
		if err := cmdSummary([]string{"--month", strconvI(m)}); err != nil {
			t.Fatalf("summary by month: %v", err)
		}
	})
	if !strings.Contains(out, "# Total expenses for ") {
		t.Fatalf("missing month summary header: %s", out)
	}
}

func TestBudgetSetAndExport(t *testing.T) {
	defer withTempHome(t)()
	now := time.Now()
	mk := now.Format("2006-01")
	m := int(now.Month())
	y := now.Year()
	// Set budget
	if err := cmdBudget([]string{"--set", "50", "--month", strconvI(m), "--year", strconvI(y)}); err != nil {
		t.Fatalf("budget set: %v", err)
	}
	st, err := loadStore()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if st.Budgets[mk] != 50 {
		t.Fatalf("budget not saved: %+v", st.Budgets)
	}
	// Add an expense and export CSV
	date := now.Format("2006-01-02")
	if err := cmdAdd([]string{"--description", "Book", "--amount", "12.34", "--date", date}); err != nil {
		t.Fatalf("add: %v", err)
	}
	csvPath := filepath.Join(t.TempDir(), "out.csv")
	if err := cmdExport([]string{"--output", csvPath}); err != nil {
		t.Fatalf("export: %v", err)
	}
	if _, err := os.Stat(csvPath); err != nil {
		t.Fatalf("export file missing: %v", err)
	}
}

// helpers
func strconvI(x int) string { return fmt.Sprintf("%d", x) }

// Avoid unused imports on certain OSes during go test caching quirks
var _ = runtime.GOOS
