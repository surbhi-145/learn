// this code is part of https://cstack.github.io/db_tutorial/

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	PAGE_SIZE       = 4096
	TABLE_MAX_PAGES = 100
	DB_DIR          = "./db"
)

type ParseError struct {
	Message string
}

func (e *ParseError) Error() string {
	return e.Message
}

type Column struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Size   uint32 `json:"size"`
	Offset uint32 `json:"offset"`
}

type Pager struct {
	File  *os.File
	Pages [TABLE_MAX_PAGES][]byte
}

type Table struct {
	Name       string   `json:"name"`
	Columns    []Column `json:"columns"`
	PrimaryKey string   `json:"primary_key"`
	RowSize    uint32   `json:"row_size"`
	NumRows    uint32   `json:"num_rows"`
	Pager      *Pager   `json:"pager"`
}

type Database struct {
	Tables map[string]*Table
}

func NewPager(tableName string) (*Pager, uint32, error) {
	filename := filepath.Join(DB_DIR, tableName+".db")
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, 0, err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, 0, err
	}
	fileSize := fileInfo.Size()
	pager := &Pager{
		File:  file,
		Pages: [TABLE_MAX_PAGES][]byte{},
	}
	return pager, uint32(fileSize), nil
}

func (p *Pager) FlushPage(pageNum uint32, size uint32) error {
	if p.Pages[pageNum] == nil {
		return fmt.Errorf("page %d is not loaded in memory", pageNum)
	}
	_, err := p.File.WriteAt(p.Pages[pageNum][:size], int64(pageNum*PAGE_SIZE))
	if err != nil {
		return err
	}
	return nil
}

func flushTable(table *Table) error {
	// store the table metadata (columns, primary key, etc.) in a separate file or in the first page of the table file
	// We can store these into JSON format.
	file, err := os.OpenFile(filepath.Join(DB_DIR, table.Name+".meta"), os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	// Serialize the table metadata to JSON
	metadata := fmt.Sprintf(`{"name":"%s","columns":[`, table.Name)
	for i, column := range table.Columns {
		metadata += fmt.Sprintf(`{"name":"%s","type":"%s","size":%d,"offset":%d}`, column.Name, column.Type, column.Size, column.Offset)
		if i < len(table.Columns)-1 {
			metadata += ","
		}
	}
	metadata += fmt.Sprintf(`],"primary_key":"%s","row_size":%d,"num_rows":%d}`, table.PrimaryKey, table.RowSize, table.NumRows)

	_, err = file.WriteString(metadata)
	if err != nil {
		return err
	}

	return nil
}

func InitializeDatabase() *Database {
	db := &Database{
		Tables: make(map[string]*Table),
	}

	// load all saved tables from the database directory
	files, err := os.ReadDir(DB_DIR)
	if err != nil {
		fmt.Println("Error reading database directory:", err)
		return db
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".db") {
			tableName := strings.TrimSuffix(file.Name(), ".db")
			pager, fileSize, err := NewPager(tableName)
			if err != nil {
				fmt.Println("Error loading table", tableName, ":", err)
				continue
			}
			tableMetadataFileName := filepath.Join(DB_DIR, tableName+".meta")
			tableMetadataFile, err := os.Open(tableMetadataFileName)
			defer tableMetadataFile.Close()
			if err != nil {
				fmt.Println("Error opening metadata file for table", tableName, ":", err)
				continue
			}

			decoder := json.NewDecoder(tableMetadataFile)
			var tableMetadata Table
			err = decoder.Decode(&tableMetadata)
			if err != nil {
				fmt.Println("Error decoding metadata for table", tableName, ":", err)
				continue
			}

			numRows := 0
			if tableMetadata.RowSize > 0 {
				numRows = int(fileSize / tableMetadata.RowSize)
			}
			table := &Table{
				Name:       tableName,
				Columns:    tableMetadata.Columns,    // Columns will be loaded from metadata
				PrimaryKey: tableMetadata.PrimaryKey, // Primary key will be loaded from metadata
				RowSize:    tableMetadata.RowSize,    // Row size will be loaded from metadata
				NumRows:    uint32(numRows),
				Pager:      pager,
			}
			db.Tables[tableName] = table
		}
	}

	return db
}

func CloseDatabase(db *Database) error {
	for _, table := range db.Tables {
		pager := table.Pager
		if pager != nil {
			for pageNum, page := range pager.Pages {
				if page != nil {
					err := pager.FlushPage(uint32(pageNum), table.RowSize)
					if err != nil {
						return err
					}
				}
			}
			err := pager.File.Close()
			if err != nil {
				return err
			}
		}
		err := flushTable(table)
		if err != nil {
			return err
		}
	}
	return nil
}

func printTable(table Table) {
	fmt.Println("Table name:", table.Name)
	fmt.Println("Columns:")
	for _, column := range table.Columns {
		fmt.Printf("  - %s (%s)\n", column.Name, column.Type)
	}
	fmt.Println("Primary key:", table.PrimaryKey)
	fmt.Println("Row size:", table.RowSize)
	fmt.Println("Number of rows:", table.NumRows)
}

func validatePrimaryKey(columns []Column, primaryKey string) *ParseError {
	for _, column := range columns {
		if column.Name == primaryKey {
			return nil
		}
	}
	return &ParseError{Message: "Primary key '" + primaryKey + "' does not match any column names."}
}

func getTableByName(db *Database, name string) *Table {
	// Placeholder implementation for demonstration purposes.
	// In a real implementation, this would look up the table in the database.
	if table, exists := db.Tables[name]; exists {
		return table
	}
	return nil
}

func calculateColumnLayout(columns []Column) (uint32, []Column) {
	var offset uint32 = 0
	for i := range columns {
		switch columns[i].Type {
		case "int":
			columns[i].Size = 4
		case "string":
			columns[i].Size = 255 // Assuming a fixed size for strings for simplicity
		default:
			columns[i].Size = 0 // Unknown type
		}
		columns[i].Offset = offset
		offset += columns[i].Size
	}
	return offset, columns
}

func parseCreateStatement(db *Database, input string) *ParseError {
	pattern := regexp.MustCompile(`\(([a-zA-Z0-9_]+),\s*([a-zA-Z0-9_]+)\)`)
	input = strings.TrimPrefix(input, "create")
	tableName, columnStrings, found := strings.Cut(input, "with")
	if !found {
		return &ParseError{Message: "Syntax error: missing 'with' keyword in create statement."}
	}
	columnStrings, primaryKey, found := strings.Cut(columnStrings, "primaryKey")
	if !found {
		return &ParseError{Message: "Syntax error: missing 'primaryKey' keyword in create statement."}
	}
	tableName = strings.TrimSpace(tableName)
	primaryKey = strings.TrimSpace(primaryKey)
	columnsString := pattern.FindAllStringSubmatch(columnStrings, -1)
	var columns []Column
	for _, match := range columnsString {
		column := Column{
			Name: strings.TrimSpace(match[1]),
			Type: strings.TrimSpace(match[2]),
		}
		columns = append(columns, column)
	}
	errs := validatePrimaryKey(columns, primaryKey)
	if errs != nil {
		return errs
	}
	offset, updatedColumns := calculateColumnLayout(columns)
	pager, fileSize, err := NewPager(tableName)
	numRows := fileSize / offset
	if err != nil {
		return &ParseError{Message: "Failed to create pager for table '" + tableName + "': " + err.Error()}
	}
	table := Table{
		Name:       tableName,
		Columns:    updatedColumns,
		PrimaryKey: primaryKey,
		RowSize:    offset,
		Pager:      pager,
		NumRows:    uint32(numRows),
	}
	printTable(table)
	if db.Tables[tableName] != nil {
		return &ParseError{Message: "Table '" + tableName + "' already exists."}
	}
	db.Tables[tableName] = &table
	return nil
}

func SerializeRow(table *Table, columns []string) ([]byte, error) {
	if len(columns) != len(table.Columns) {
		return nil, fmt.Errorf("number of columns does not match table definition")
	}

	row := make([]byte, table.RowSize)
	for i, column := range columns {
		switch table.Columns[i].Type {
		case "int":
			value, err := strconv.Atoi(column)
			if err != nil {
				return nil, fmt.Errorf("invalid int value for column '%s': %v", table.Columns[i].Name, err)
			}
			offset := table.Columns[i].Offset
			row[offset] = byte(value >> 24)
			row[offset+1] = byte(value >> 16)
			row[offset+2] = byte(value >> 8)
			row[offset+3] = byte(value)
		case "string":
			if len(column) > int(table.Columns[i].Size) {
				return nil, fmt.Errorf("string value for column '%s' exceeds maximum size", table.Columns[i].Name)
			}
			offset := table.Columns[i].Offset
			copy(row[offset:], []byte(column))
		default:
			return nil, fmt.Errorf("unknown type for column '%s'", table.Columns[i].Name)
		}
	}
	return row, nil
}

func DeserializeRow(table *Table, row []byte) ([]string, error) {
	if len(row) != int(table.RowSize) {
		return nil, fmt.Errorf("row size does not match table definition")
	}

	columns := make([]string, len(table.Columns))
	for i, column := range table.Columns {
		offset := column.Offset
		switch column.Type {
		case "int":
			value := int(row[offset])<<24 | int(row[offset+1])<<16 | int(row[offset+2])<<8 | int(row[offset+3])
			columns[i] = strconv.Itoa(value)
		case "string":
			strValue := string(row[offset : offset+column.Size])
			columns[i] = strValue
		default:
			return nil, fmt.Errorf("unknown type for column '%s'", column.Name)
		}
	}
	return columns, nil
}

func rowSlot(table *Table, rowNum uint32) ([]byte, uint32) {
	rowsPerPage := PAGE_SIZE / table.RowSize
	pageNum := rowNum / rowsPerPage
	rowOffset := rowNum % rowsPerPage
	byteOffset := rowOffset * table.RowSize

	// Allocate page memory only when accessed (lazy allocation)
	// Cache miss
	if table.Pager.Pages[pageNum] == nil {
		table.Pager.Pages[pageNum] = make([]byte, PAGE_SIZE)

		_, err := table.Pager.File.ReadAt(table.Pager.Pages[pageNum], int64(pageNum*PAGE_SIZE))
		// read from disk into memory, if the page is not already in memory
		if err != nil && err != io.EOF {
			fmt.Println("Error reading page from file:", err)
		}
	}

	return table.Pager.Pages[pageNum], byteOffset
}

func parseInsertStatement(db *Database, input string) *ParseError {
	input = strings.TrimPrefix(input, "insert")
	columnsString, tableName, found := strings.Cut(input, "into")
	if !found {
		return &ParseError{Message: "Syntax error: missing 'into' keyword in insert statement."}
	}
	tableName = strings.TrimSpace(tableName)
	table := getTableByName(db, tableName)
	if table == nil {
		return &ParseError{Message: "Table '" + tableName + "' does not exist."}
	}
	columnsString = strings.TrimSpace(columnsString)
	columns := strings.Split(columnsString, ",")
	if len(columns) != len(table.Columns) {
		return &ParseError{Message: "Syntax error: incorrect number of columns specified in insert statement."}
	}
	for i, column := range columns {
		columns[i] = strings.TrimSpace(column)
		switch table.Columns[i].Type {
		case "int":
			if _, err := strconv.Atoi(column); err != nil {
				return &ParseError{Message: "Type error: expected int for column '" + table.Columns[i].Name + "'."}
			}
		case "string":
			if !strings.HasPrefix(column, "\"") || !strings.HasSuffix(column, "\"") {
				return &ParseError{Message: "Type error: expected string for column '" + table.Columns[i].Name + "'."}
			}
		default:
			return &ParseError{Message: "Unknown type for column '" + table.Columns[i].Name + "'."}
		}
	}
	fmt.Println("Parsed insert statement:")
	fmt.Println("Table name:", tableName)
	fmt.Println("Columns:", columns)

	rowsPerPage := PAGE_SIZE / table.RowSize
	if table.NumRows >= uint32(rowsPerPage*TABLE_MAX_PAGES) {
		return &ParseError{Message: "Table '" + tableName + "' is full."}
	}

	page, byteOffset := rowSlot(table, table.NumRows)
	row, err := SerializeRow(table, columns)
	if err != nil {
		return &ParseError{Message: err.Error()}
	}
	copy(page[byteOffset:], row)
	table.NumRows++
	printTable(*table)
	return nil

}

func parseSelectStatement(db *Database, input string) *ParseError {
	input = strings.TrimPrefix(input, "select")
	tableName := strings.TrimSpace(input)
	table := getTableByName(db, tableName)
	if table == nil {
		return &ParseError{Message: "Table '" + tableName + "' does not exist."}
	}

	fmt.Println("Parsed select statement:")
	fmt.Println("Table name:", tableName)

	for rowNum := uint32(0); rowNum < table.NumRows; rowNum++ {
		page, byteOffset := rowSlot(table, rowNum)
		rowData := page[byteOffset : byteOffset+table.RowSize]
		columns, err := DeserializeRow(table, rowData)
		if err != nil {
			return &ParseError{Message: err.Error()}
		}
		fmt.Println("Row", rowNum+1, ":", columns)
	}

	return nil
}

func metaCommand(db *Database, input string) {
	switch input {
	case ".exit":
		err := CloseDatabase(db)
		if err != nil {
			fmt.Println("Error closing database:", err)
		}
		os.Exit(0)
	default:
		fmt.Println("Unrecognized command '" + input + "'.")
	}
}

func executeStatement(db *Database, input string) {
	if strings.HasPrefix(input, "create") {
		err := parseCreateStatement(db, input)
		if err != nil {
			fmt.Println(err.Error())
		}
	} else if strings.HasPrefix(input, "insert") {
		err := parseInsertStatement(db, input)
		if err != nil {
			fmt.Println(err.Error())
		}
	} else if strings.HasPrefix(input, "select") {
		err := parseSelectStatement(db, input)
		if err != nil {
			fmt.Println(err.Error())
		}
	} else {
		fmt.Println("Unrecognized keyword at start of '" + input + "'.")
	}
}

func main() {
	db := InitializeDatabase()
	fmt.Print("> ")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		if strings.HasPrefix(input, ".") {
			metaCommand(db, input)
		} else {
			executeStatement(db, input)
		}
		fmt.Print("> ")
	}
}
