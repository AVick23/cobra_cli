package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/c-bata/go-prompt"
	_ "github.com/mattn/go-sqlite3"
)

var selectDB string
var db *sql.DB
var dbFilePath string

func completer(d prompt.Document) []prompt.Suggest {
	if selectDB == "" {
		return prompt.FilterHasPrefix([]prompt.Suggest{
			{Text: "sqlite", Description: "Выберите работу с SQLite"},
			{Text: "exit", Description: "Завершите работу программы"},
		}, d.GetWordBeforeCursor(), true)
	} else if selectDB == "sqlite" {
		return prompt.FilterHasPrefix([]prompt.Suggest{
			{Text: "create", Description: "Создать таблицу"},
			{Text: "structure", Description: "Показать структуру таблицы"},
			{Text: "newdb", Description: "Создать новую базу данных"},
			{Text: "name", Description: "Узнать название бд с которой работаете в данный момент"},
			{Text: "exit", Description: "Завершите работу программы"},
		}, d.GetWordBeforeCursor(), true)
	}

	return nil
}

func createDatabase() {
	dir := "databases"
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		fmt.Printf("Ошибка при создании директории: %v\n", err)
		return
	}

	fmt.Print("Введите имя новой базы данных: ")
	input := prompt.Input("> ", completer)
	dbName := input

	if !strings.HasSuffix(dbName, ".db") {
		dbName += ".db"
	}

	dbPath := filepath.Join(dir, dbName)

	if _, err := os.Stat(dbPath); err == nil {
		fmt.Println("База данных с таким именем уже существует.")
		return
	}

	newDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		fmt.Printf("Ошибка при создании базы данных: %v\n", err)
		return
	}
	defer newDB.Close()

	_, err = newDB.Exec("CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY);")
	if err != nil {
		fmt.Printf("Ошибка при инициализации базы данных: %v\n", err)
		return
	}

	fmt.Printf("База данных '%s' успешно создана в директории '%s'.\n", dbName, dir)
}

func connectDB() {
	dir := "databases"
	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("Ошибка при чтении директории: %v\n", err)
		return
	}

	var dbFiles []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".db" {
			dbFiles = append(dbFiles, filepath.Join(dir, file.Name()))
		}
	}

	if len(dbFiles) == 0 {
		fmt.Println("Нет доступных баз данных в директории.")
		return
	}

	fmt.Println("Выберите базу данных для подключения:")
	for i, dbFile := range dbFiles {
		fmt.Printf("[%d] %s\n", i+1, dbFile)
	}

	var choice int
	fmt.Print("Введите номер базы данных для подключения: ")
	input := prompt.Input("> ", completer)
	fmt.Sscanf(input, "%d", &choice)

	if choice < 1 || choice > len(dbFiles) {
		fmt.Println("Некорректный выбор.")
		return
	}

	dbFilePath = dbFiles[choice-1]
	fmt.Printf("\nПодключение к базе данных: %s\n", dbFilePath)

	db, err = sql.Open("sqlite3", dbFilePath)
	if err != nil {
		log.Fatalf("Ошибка при подключении к базе данных: %v\n", err)
		return
	}

	err = db.Ping()
	if err != nil {
		log.Fatalf("Ошибка при проверке подключения: %v\n", err)
		db = nil
		return
	}

	fmt.Println("Успешное подключение к базе данных.")
}

func closeDB() {
	if db != nil {
		err := db.Close()
		if err != nil {
			fmt.Printf("Ошибка при закрытии базы данных: %v\n", err)
		} else {
			fmt.Println("Подключение к базе данных закрыто.")
		}
		db = nil
	} else {
		fmt.Println("Подключение к базе данных уже закрыто или не было установлено.")
	}
}

func getDBName() string {
	if dbFilePath == "" {
		return "База данных не выбрана"
	}
	return fmt.Sprintf("Вы сейчас работаете с базой данных: %s", dbFilePath)
}

func listTables() ([]string, error) {
	query := `SELECT name FROM sqlite_master WHERE type='table';`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении списка таблиц: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("ошибка при сканировании названия таблицы: %w", err)
		}
		tables = append(tables, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка при обработке строк: %w", err)
	}

	return tables, nil
}

func selectTable(tables []string) (string, error) {
	if len(tables) == 0 {
		return "", fmt.Errorf("нет доступных таблиц для выбора")
	}

	fmt.Println("Список таблиц:")
	for i, table := range tables {
		fmt.Printf("[%d] %s\n", i+1, table)
	}

	var choice int
	fmt.Print("Введите номер таблицы для отображения её структуры: ")
	input := prompt.Input("> ", completer)
	fmt.Sscanf(input, "%d", &choice)

	if choice < 1 || choice > len(tables) {
		return "", fmt.Errorf("некорректный выбор")
	}

	return tables[choice-1], nil
}

func printTableStructure(tableName string) error {
	query := fmt.Sprintf("PRAGMA table_info(%s);", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("ошибка при получении структуры таблицы %s: %w", tableName, err)
	}
	defer rows.Close()

	fmt.Printf("\nСтруктура таблицы %s:\n", tableName)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("%-20s %-15s %-10s %-10s %-15s\n", "Поле", "Тип", "Не NULL", "Ключ", "По умолчанию")
	fmt.Println(strings.Repeat("-", 60))

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("ошибка при сканировании структуры таблицы %s: %w", tableName, err)
		}

		fmt.Printf("%-20s %-15s %-10d %-10d %-15v\n", name, ctype, notnull, pk, dfltValue.String)
	}

	fmt.Println(strings.Repeat("-", 60))
	return rows.Err()
}

func executor(in string) {
	in = strings.TrimSpace(in)

	if selectDB == "" {
		switch in {
		case "sqlite":
			fmt.Println("Вы подключились к SQLite")
			connectDB()
			selectDB = "sqlite"
		case "exit":
			fmt.Println("Выход из программы.")
			return
		default:
			fmt.Println("Неизвестная команда:", in)
		}
	} else if selectDB == "sqlite" {
		switch in {
		case "name":
			name := getDBName()
			fmt.Println(name)
		case "newdb":
			createDatabase()
		case "structure":
			tables, err := listTables()
			if err != nil {
				fmt.Println("Ошибка при получении списка таблиц:", err)
				return
			}

			tableName, err := selectTable(tables)
			if err != nil {
				fmt.Println("Ошибка при выборе таблицы:", err)
				return
			}

			if err := printTableStructure(tableName); err != nil {
				fmt.Println("Ошибка при получении структуры таблицы:", err)
			}
		case "exit":
			fmt.Println("Выход из программы.")
			closeDB()
			return
		default:
			fmt.Println("Неизвестная команда для SQLite:", in)
		}
	}
}

func main() {
	fmt.Println("Введите команды sqlite или exit")
	for {
		input := prompt.Input("> ", completer)
		if input == "exit" {
			break
		}
		executor(input)
	}
	closeDB()
}
