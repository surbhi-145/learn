// this code is part of https://cstack.github.io/db_tutorial/

package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	PAGE_SIZE       = 4096
	TABLE_MAX_PAGES = 100
)

type ParseError struct {
	Message string
}

func (e *ParseError) Error() string {
	return e.Message
}

type Column struct {
	Name   string
	Type   string
	Size   uint32
	Offset uint32
}

type Table struct {
	Name       string
	Columns    []Column
	PrimaryKey string
	RowSize    uint32
	NumRows    uint32
	Pages      [TABLE_MAX_PAGES][]byte
}

type Database struct {
	Tables map[string]*Table
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
	err := validatePrimaryKey(columns, primaryKey)
	if err != nil {
		return err
	}
	offset, updatedColumns := calculateColumnLayout(columns)
	table := Table{
		Name:       tableName,
		Columns:    updatedColumns,
		PrimaryKey: primaryKey,
		RowSize:    offset,
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
	if table.Pages[pageNum] == nil {
		table.Pages[pageNum] = make([]byte, PAGE_SIZE)
	}

	return table.Pages[pageNum], byteOffset
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

func metaCommand(input string) {
	switch input {
	case ".exit":
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
	db := Database{
		Tables: make(map[string]*Table),
	}
	fmt.Print("> ")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		if strings.HasPrefix(input, ".") {
			metaCommand(input)
		} else {
			executeStatement(&db, input)
		}
		fmt.Print("> ")
	}
}
