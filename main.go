package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ===== Data structures =====

type Expense struct {
	ID          int       `json:"id"`
	Date        string    `json:"date"` // YYYY-MM-DD
	Description string    `json:"description"`
	Amount      float64   `json:"amount"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Store struct {
	NextID   int                `json:"next_id"`
	Expenses []Expense          `json:"expenses"`
	Budgets  map[string]float64 `json:"budgets"` // key: YYYY-MM
}

// ===== Constants & paths =====

const (
	appName        = "expense-tracker"
	dataFileName   = "expenses.json"
	defaultCat     = "General"
	dateLayout     = "2006-01-02"
	monthKeyLayout = "2006-01"
)

func dataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "."+appName), nil
}

func dataPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, dataFileName), nil
}

func ensureDir() error {
	dir, err := dataDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

// ===== Load & save =====

func loadStore() (*Store, error) {
	path, err := dataPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Store{NextID: 1, Budgets: map[string]float64{}}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var st Store
	if err := json.NewDecoder(f).Decode(&st); err != nil {
		return nil, err
	}
	if st.Budgets == nil {
		st.Budgets = map[string]float64{}
	}
	if st.NextID == 0 {
		st.NextID = 1
	}
	return &st, nil
}

func (st *Store) save() error {
	if err := ensureDir(); err != nil {
		return err
	}
	path, err := dataPath()
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(st); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// ===== Helpers =====

func parseDateOrToday(s string) (string, error) {
	if strings.TrimSpace(s) == "" {
		return time.Now().Format(dateLayout), nil
	}
	if _, err := time.Parse(dateLayout, s); err != nil {
		return "", fmt.Errorf("invalid date %q, expected YYYY-MM-DD", s)
	}
	return s, nil
}

func monthKeyFromDate(date string) (string, error) {
	t, err := time.Parse(dateLayout, date)
	if err != nil {
		return "", err
	}
	return t.Format(monthKeyLayout), nil
}

func (st *Store) sumForMonth(key string, category string) float64 {
	total := 0.0
	for _, e := range st.Expenses {
		mk, err := monthKeyFromDate(e.Date)
		if err != nil {
			continue
		}
		if mk == key && (category == "" || strings.EqualFold(category, e.Category)) {
			total += e.Amount
		}
	}
	return total
}

func (st *Store) findByID(id int) (*Expense, int) {
	for i := range st.Expenses {
		if st.Expenses[i].ID == id {
			return &st.Expenses[i], i
		}
	}
	return nil, -1
}

func printTable(expenses []Expense, w io.Writer) {
	fmt.Fprintln(w, "# ID  Date        Description                      Category        Amount")
	fmt.Fprintln(w, "# --- ----------- -------------------------------- ---------------- ---------")
	for _, e := range expenses {
		desc := e.Description
		if len(desc) > 32 {
			desc = desc[:29] + "..."
		}
		cat := e.Category
		if len(cat) > 14 {
			cat = cat[:11] + "..."
		}
		fmt.Fprintf(w, "# %-3d %-11s %-32s %-14s $%.2f\n", e.ID, e.Date, desc, cat, e.Amount)
	}
}

func warnIfBudgetExceeded(st *Store, date string, category string) {
	mk, err := monthKeyFromDate(date)
	if err != nil {
		return
	}
	budget, ok := st.Budgets[mk]
	if !ok || budget <= 0 {
		return
	}
	total := st.sumForMonth(mk, category)
	if total > budget {
		fmt.Printf("# Warning: budget for %s exceeded! Budget: $%.2f, Spent: $%.2f\n", mk, budget, total)
	}
}

// ===== Commands =====

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	desc := fs.String("description", "", "expense description (required)")
	amount := fs.Float64("amount", 0, "expense amount (required, > 0)")
	date := fs.String("date", "", "date in YYYY-MM-DD (default: today)")
	cat := fs.String("category", defaultCat, "category name")
	fs.Parse(args)

	if strings.TrimSpace(*desc) == "" {
		return errors.New("--description is required")
	}
	if *amount <= 0 {
		return errors.New("--amount must be > 0")
	}
	d, err := parseDateOrToday(*date)
	if err != nil {
		return err
	}

	st, err := loadStore()
	if err != nil {
		return err
	}
	e := Expense{ID: st.NextID, Date: d, Description: strings.TrimSpace(*desc), Amount: *amount, Category: strings.TrimSpace(*cat), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	st.Expenses = append(st.Expenses, e)
	st.NextID++
	if err := st.save(); err != nil {
		return err
	}
	fmt.Printf("# Expense added successfully (ID: %d)\n", e.ID)
	warnIfBudgetExceeded(st, e.Date, "")
	return nil
}

// floatFlag tracks whether a float flag was explicitly set.
type floatFlag struct {
	set bool
	val float64
}

func (f *floatFlag) String() string { return fmt.Sprintf("%v", f.val) }
func (f *floatFlag) Set(s string) error {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	f.val = v
	f.set = true
	return nil
}

func cmdUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	id := fs.Int("id", 0, "expense ID (required)")
	desc := fs.String("description", "", "new description")
	var amt floatFlag
	fs.Var(&amt, "amount", "new amount (number > 0)")
	date := fs.String("date", "", "new date (YYYY-MM-DD)")
	cat := fs.String("category", "", "new category")
	fs.Parse(args)
	if *id <= 0 {
		return errors.New("--id is required")
	}

	st, err := loadStore()
	if err != nil {
		return err
	}
	e, _ := st.findByID(*id)
	if e == nil {
		return fmt.Errorf("expense with ID %d not found", *id)
	}

	if strings.TrimSpace(*desc) != "" {
		e.Description = strings.TrimSpace(*desc)
	}
	if amt.set {
		if amt.val <= 0 {
			return errors.New("--amount must be a positive number")
		}
		e.Amount = amt.val
	}
	if strings.TrimSpace(*date) != "" {
		if _, err := time.Parse(dateLayout, *date); err != nil {
			return fmt.Errorf("invalid --date: %v", err)
		}
		e.Date = *date
	}
	if strings.TrimSpace(*cat) != "" {
		e.Category = strings.TrimSpace(*cat)
	}
	e.UpdatedAt = time.Now()

	if err := st.save(); err != nil {
		return err
	}
	fmt.Println("# Expense updated successfully")
	warnIfBudgetExceeded(st, e.Date, "")
	return nil
}

func cmdDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	id := fs.Int("id", 0, "expense ID (required)")
	fs.Parse(args)
	if *id <= 0 {
		return errors.New("--id is required")
	}

	st, err := loadStore()
	if err != nil {
		return err
	}
	_, idx := st.findByID(*id)
	if idx < 0 {
		return fmt.Errorf("expense with ID %d not found", *id)
	}
	st.Expenses = append(st.Expenses[:idx], st.Expenses[idx+1:]...)
	if err := st.save(); err != nil {
		return err
	}
	fmt.Println("# Expense deleted successfully")
	return nil
}

func filterExpenses(st *Store, month int, year int, category string) []Expense {
	var out []Expense
	for _, e := range st.Expenses {
		if category != "" && !strings.EqualFold(category, e.Category) {
			continue
		}
		if month > 0 || year > 0 {
			t, err := time.Parse(dateLayout, e.Date)
			if err != nil {
				continue
			}
			if year > 0 && t.Year() != year {
				continue
			}
			if month > 0 && int(t.Month()) != month {
				continue
			}
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Date < out[j].Date || (out[i].Date == out[j].Date && out[i].ID < out[j].ID)
	})
	return out
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	category := fs.String("category", "", "filter by category (case-insensitive)")
	month := fs.Int("month", 0, "filter by month (1-12) of current year or with --year")
	year := fs.Int("year", 0, "filter by year (e.g., 2025). default: all years or current when --month is set")
	fs.Parse(args)

	st, err := loadStore()
	if err != nil {
		return err
	}
	if *month != 0 && (*month < 1 || *month > 12) {
		return errors.New("--month must be 1-12")
	}
	if *month > 0 && *year == 0 {
		*year = time.Now().Year()
	}
	items := filterExpenses(st, *month, *year, strings.TrimSpace(*category))
	printTable(items, os.Stdout)
	return nil
}

func cmdSummary(args []string) error {
	fs := flag.NewFlagSet("summary", flag.ExitOnError)
	month := fs.Int("month", 0, "month (1-12) of current year")
	category := fs.String("category", "", "filter by category")
	fs.Parse(args)

	st, err := loadStore()
	if err != nil {
		return err
	}
	if *month == 0 {
		total := 0.0
		for _, e := range st.Expenses {
			if *category == "" || strings.EqualFold(*category, e.Category) {
				total += e.Amount
			}
		}
		if *category == "" {
			fmt.Printf("# Total expenses: $%.2f\n", total)
		} else {
			fmt.Printf("# Total expenses (%s): $%.2f\n", *category, total)
		}
		return nil
	}
	if *month < 1 || *month > 12 {
		return errors.New("--month must be 1-12")
	}
	year := time.Now().Year()
	mk := fmt.Sprintf("%04d-%02d", year, *month)
	total := st.sumForMonth(mk, strings.TrimSpace(*category))
	monName := time.Month(*month).String()
	if strings.TrimSpace(*category) == "" {
		fmt.Printf("# Total expenses for %s: $%.2f\n", monName, total)
	} else {
		fmt.Printf("# Total expenses for %s (%s): $%.2f\n", monName, *category, total)
	}
	return nil
}

func cmdBudget(args []string) error {
	fs := flag.NewFlagSet("budget", flag.ExitOnError)
	set := fs.Float64("set", -1, "set monthly budget (e.g., 500)")
	month := fs.Int("month", 0, "month (1-12), default: current month")
	year := fs.Int("year", 0, "year, default: current year")
	fs.Parse(args)

	if *month == 0 {
		*month = int(time.Now().Month())
	}
	if *year == 0 {
		*year = time.Now().Year()
	}
	if *month < 1 || *month > 12 {
		return errors.New("--month must be 1-12")
	}
	mk := fmt.Sprintf("%04d-%02d", *year, *month)

	st, err := loadStore()
	if err != nil {
		return err
	}
	if *set >= 0 {
		st.Budgets[mk] = *set
		if err := st.save(); err != nil {
			return err
		}
		fmt.Printf("# Budget for %s set to $%.2f\n", mk, *set)
		return nil
	}
	budget, ok := st.Budgets[mk]
	if !ok || budget <= 0 {
		fmt.Printf("# No budget set for %s\n", mk)
		return nil
	}
	fmt.Printf("# Budget for %s: $%.2f\n", mk, budget)
	spent := st.sumForMonth(mk, "")
	fmt.Printf("# Spent: $%.2f, Remaining: $%.2f\n", spent, budget-spent)
	return nil
}

func cmdExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	out := fs.String("output", "expenses.csv", "output CSV file path")
	category := fs.String("category", "", "filter by category")
	month := fs.Int("month", 0, "filter month (1-12) of current year or with --year")
	year := fs.Int("year", 0, "filter year")
	fs.Parse(args)

	st, err := loadStore()
	if err != nil {
		return err
	}
	if *month != 0 && (*month < 1 || *month > 12) {
		return errors.New("--month must be 1-12")
	}
	if *month > 0 && *year == 0 {
		*year = time.Now().Year()
	}
	items := filterExpenses(st, *month, *year, strings.TrimSpace(*category))

	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	_ = w.Write([]string{"id", "date", "description", "category", "amount"})
	for _, e := range items {
		_ = w.Write([]string{strconv.Itoa(e.ID), e.Date, e.Description, e.Category, fmt.Sprintf("%.2f", e.Amount)})
	}
	if err := w.Error(); err != nil {
		return err
	}
	fmt.Printf("# Exported %d rows to %s\n", len(items), *out)
	return nil
}

// ===== CLI scaffolding =====

func usage() {
	fmt.Printf("%s — simple CLI expense tracker\n\n", appName)
	fmt.Println("Usage:")
	fmt.Println("  expense-tracker <command> [options]")
	fmt.Println("Commands:")
	fmt.Println("  add         --description <text> --amount <number> [--date YYYY-MM-DD] [--category NAME]")
	fmt.Println("  update      --id <n> [--description <text>] [--amount <number>] [--date YYYY-MM-DD] [--category NAME]")
	fmt.Println("  delete      --id <n>")
	fmt.Println("  list        [--category NAME] [--month 1-12] [--year YYYY]")
	fmt.Println("  summary     [--month 1-12] [--category NAME]")
	fmt.Println("  budget      [--set <number>] [--month 1-12] [--year YYYY]")
	fmt.Println("  export      [--output FILE] [--category NAME] [--month 1-12] [--year YYYY]")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  expense-tracker add --description \"Lunch\" --amount 20")
	fmt.Println("  expense-tracker list")
	fmt.Println("  expense-tracker summary --month 8")
	fmt.Println("  expense-tracker budget --set 500 --month 8 --year 2025")
	fmt.Println("  expense-tracker export --output my-expenses.csv --month 8")
}

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	cmd := strings.ToLower(os.Args[1])
	args := os.Args[2:]
	var err error
	switch cmd {
	case "add":
		err = cmdAdd(args)
	case "update":
		err = cmdUpdate(args)
	case "delete":
		err = cmdDelete(args)
	case "list":
		err = cmdList(args)
	case "summary":
		err = cmdSummary(args)
	case "budget":
		err = cmdBudget(args)
	case "export":
		err = cmdExport(args)
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Printf("Unknown command: %s\n\n", cmd)
		usage()
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
