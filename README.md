# 💰 Expense Tracker (Go CLI)

A simple command-line **Expense Tracker** 📝 written in Go to help you manage your finances efficiently.

Project idea from [roadmap.sh](https://roadmap.sh/projects/expense-tracker)

## ✨ Features

- ➕ Add an expense with description, amount, date, and category.
- ✏️ Update an existing expense.
- ❌ Delete an expense.
- 📋 View all expenses in a table.
- 📊 View a summary of total expenses.
- 📅 View a summary for a specific month (current year).
- 💵 Set and check monthly budgets (⚠️ warns when exceeded).
- 📤 Export expenses to a CSV file.

## ⚙️ Installation

1. **Clone the repository:**
```bash
git clone https://github.com/yourusername/expense-tracker-go.git
cd expense-tracker-go
```

2. **Build the binary:**
```bash
go build -o expense-tracker
```

3. **Move it to your PATH** (optional):
```bash
mv expense-tracker /usr/local/bin/
```
If you do **not** move it to your PATH, you will need to run it with `./` from the build directory:
```bash
./expense-tracker <command> [options]
```

## 🚀 Usage

```bash
expense-tracker <command> [options]
# or if not in PATH
./expense-tracker <command> [options]
```

### 📌 Commands

#### ➕ Add an expense
```bash
expense-tracker add --description "Lunch" --amount 20 --date 2025-08-09 --category Food
```

#### ✏️ Update an expense
```bash
expense-tracker update --id 1 --amount 25.5 --category Dining
```

#### ❌ Delete an expense
```bash
expense-tracker delete --id 2
```

#### 📋 List expenses
```bash
expense-tracker list
expense-tracker list --category Food
expense-tracker list --month 8 --year 2025
```

#### 📊 Summary
```bash
expense-tracker summary
expense-tracker summary --month 8
expense-tracker summary --category Food
```

#### 💵 Monthly budget
```bash
expense-tracker budget --set 500 --month 8 --year 2025
expense-tracker budget --month 8 --year 2025
```

#### 📤 Export to CSV
```bash
expense-tracker export --output expenses.csv
```

## 📂 Data Storage

Expenses are saved in a JSON file at:
```
~/.expense-tracker/expenses.json
```

## 🧪 Running Tests

```bash
go test -v
```

## 📜 License

MIT License - see [LICENSE](LICENSE) for details.
