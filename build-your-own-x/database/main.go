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

const (
	NODE_INTERNAL uint8 = 0
	NODE_LEAF     uint8 = 1
)

// Common Header Layout Constants (in bytes)
const (
	NODE_TYPE_OFFSET         = 0 // 1 byte
	NODE_IS_ROOT_OFFSET      = 1 // 1 byte
	POINTER_TO_PARENT_OFFSET = 2 // 4 bytes (uint32)
	NUM_CELLS_OFFSET         = 6 // 4 bytes (uint32)
	COMMON_NODE_HEADER_SIZE  = 10
)

// Leaf Node Constants
const (
	LEAF_NODE_KEY_SIZE    = 4 // uint32 primary key
	LEAF_NODE_HEADER_SIZE = COMMON_NODE_HEADER_SIZE
)

// Internal Node Constants
const (
	INTERNAL_NODE_CHILD_SIZE  = 4 // uint32 page number
	INTERNAL_NODE_KEY_SIZE    = 4 // uint32 key
	INTERNAL_NODE_HEADER_SIZE = COMMON_NODE_HEADER_SIZE
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
	Name        string   `json:"name"`
	Columns     []Column `json:"columns"`
	PrimaryKey  string   `json:"primary_key"`
	RowSize     uint32   `json:"row_size"`
	RootPageNum uint32   `json:"root_page_num"`
	Pager       *Pager   `json:"pager"`
}

type Database struct {
	Tables map[string]*Table
}

// ==========================================
// BYTE READERS / WRITERS (Big Endian)
// ==========================================

func getUint32(page []byte, offset uint32) uint32 {
	return uint32(page[offset])<<24 |
		uint32(page[offset+1])<<16 |
		uint32(page[offset+2])<<8 |
		uint32(page[offset+3])
}

func putUint32(page []byte, offset uint32, value uint32) {
	page[offset] = byte(value >> 24)
	page[offset+1] = byte(value >> 16)
	page[offset+2] = byte(value >> 8)
	page[offset+3] = byte(value)
}

func getNodeType(page []byte) uint8 {
	return page[NODE_TYPE_OFFSET]
}

func setNodeType(page []byte, nodeType uint8) {
	page[NODE_TYPE_OFFSET] = nodeType
}

func isNodeRoot(page []byte) bool {
	return page[NODE_IS_ROOT_OFFSET] == 1
}

func setNodeRoot(page []byte, isRoot bool) {
	if isRoot {
		page[NODE_IS_ROOT_OFFSET] = 1
	} else {
		page[NODE_IS_ROOT_OFFSET] = 0
	}
}

func getNumCells(page []byte) uint32 {
	return getUint32(page, NUM_CELLS_OFFSET)
}

func setNumCells(page []byte, numCells uint32) {
	putUint32(page, NUM_CELLS_OFFSET, numCells)
}

func getParentPointer(page []byte) uint32 {
	return getUint32(page, POINTER_TO_PARENT_OFFSET)
}

func setParentPointer(page []byte, parentPageNum uint32) {
	putUint32(page, POINTER_TO_PARENT_OFFSET, parentPageNum)
}

// ==========================================
// LEAF NODE HELPERS
// ==========================================

func leafNodeCellSize(table *Table) uint32 {
	return LEAF_NODE_KEY_SIZE + table.RowSize
}

func leafNodeCellOffset(cellNum uint32, table *Table) uint32 {
	return LEAF_NODE_HEADER_SIZE + (cellNum * leafNodeCellSize(table))
}

func leafNodeGetKey(page []byte, cellNum uint32, table *Table) uint32 {
	return getUint32(page, leafNodeCellOffset(cellNum, table))
}

func leafNodeGetValue(page []byte, cellNum uint32, table *Table) []byte {
	cellOffset := leafNodeCellOffset(cellNum, table)
	valueOffset := cellOffset + LEAF_NODE_KEY_SIZE
	return page[valueOffset : valueOffset+table.RowSize]
}

func leafNodeFind(table *Table, page []byte, key uint32) (uint32, bool) {
	numCells := getNumCells(page)
	low := uint32(0)
	high := numCells

	for low < high {
		mid := (low + high) / 2
		midKey := leafNodeGetKey(page, mid, table)
		if midKey == key {
			return mid, true
		} else if midKey < key {
			low = mid + 1
		} else {
			high = mid
		}
	}
	return low, false
}

func leafNodeInsert(table *Table, page []byte, key uint32, row []byte) {
	numCells := getNumCells(page)
	index, _ := leafNodeFind(table, page, key)

	cellBytesSize := leafNodeCellSize(table)
	if index < numCells {
		srcOffset := leafNodeCellOffset(index, table)
		dstOffset := leafNodeCellOffset(index+1, table)
		bytesToMove := (numCells - index) * cellBytesSize
		copy(page[dstOffset:dstOffset+bytesToMove], page[srcOffset:srcOffset+bytesToMove])
	}

	// Write key and row
	cellOffset := leafNodeCellOffset(index, table)
	putUint32(page, cellOffset, key)
	copy(page[cellOffset+LEAF_NODE_KEY_SIZE:], row)

	setNumCells(page, numCells+1)
}

// ==========================================
// INTERNAL NODE HELPERS
// ==========================================

func internalNodeCellSize() uint32 {
	return INTERNAL_NODE_CHILD_SIZE + INTERNAL_NODE_KEY_SIZE
}

func internalNodeCellOffset(cellIndex uint32) uint32 {
	return INTERNAL_NODE_HEADER_SIZE + (cellIndex * internalNodeCellSize())
}

func internalNodeChild(page []byte, childIndex uint32) uint32 {
	return getUint32(page, internalNodeCellOffset(childIndex))
}

func internalNodeKey(page []byte, keyIndex uint32) uint32 {
	return getUint32(page, internalNodeCellOffset(keyIndex)+INTERNAL_NODE_CHILD_SIZE)
}

func internalNodeSetChild(page []byte, childIndex uint32, childPageNum uint32) {
	putUint32(page, internalNodeCellOffset(childIndex), childPageNum)
}

func internalNodeSetKey(page []byte, keyIndex uint32, key uint32) {
	putUint32(page, internalNodeCellOffset(keyIndex)+INTERNAL_NODE_CHILD_SIZE, key)
}

func internalNodeFindChild(page []byte, key uint32) uint32 {
	numCells := getNumCells(page)
	low := uint32(0)
	high := numCells

	for low < high {
		mid := (low + high) / 2
		midKey := internalNodeKey(page, mid)
		if midKey >= key {
			high = mid
		} else {
			low = mid + 1
		}
	}
	return low
}

// ==========================================
// B-TREE TRAVERSAL & SPLITTING
// ==========================================

func findLeafNode(table *Table, pageNum uint32, key uint32) []byte {
	page := table.Pager.Pages[pageNum]
	if page == nil {
		page = make([]byte, PAGE_SIZE)
		_, err := table.Pager.File.ReadAt(page, int64(pageNum*PAGE_SIZE))
		if err != nil && err != io.EOF {
			fmt.Println("Error reading page from file:", err)
		}
		table.Pager.Pages[pageNum] = page
	}

	nodeType := getNodeType(page)
	if nodeType == NODE_LEAF {
		return page
	} else {
		childIndex := internalNodeFindChild(page, key)
		childPageNum := internalNodeChild(page, childIndex)
		return findLeafNode(table, childPageNum, key)
	}
}

func allocateNewPage(pager *Pager) uint32 {
	for i := uint32(1); i < TABLE_MAX_PAGES; i++ {
		if pager.Pages[i] == nil {
			pager.Pages[i] = make([]byte, PAGE_SIZE)
			return i
		}
	}
	panic("Database overflow: Too many pages allocated!")
}

func createNewRootAfterLeafSplit(table *Table, leftChildPageNum uint32, rightChildPageNum uint32, dividingKey uint32) {
	rootPageNum := allocateNewPage(table.Pager)
	rootPage := table.Pager.Pages[rootPageNum]

	setNodeType(rootPage, NODE_INTERNAL)
	setNodeRoot(rootPage, true)
	setNumCells(rootPage, 2)
	setParentPointer(rootPage, 0)

	internalNodeSetChild(rootPage, 0, leftChildPageNum)
	internalNodeSetKey(rootPage, 0, dividingKey)
	internalNodeSetChild(rootPage, 1, rightChildPageNum)

	table.RootPageNum = rootPageNum

	// Update children parent pointers
	setParentPointer(table.Pager.Pages[leftChildPageNum], rootPageNum)
	setParentPointer(table.Pager.Pages[rightChildPageNum], rootPageNum)

	// Demote old root
	setNodeRoot(table.Pager.Pages[leftChildPageNum], false)
}

func leafNodeSplitAndInsert(table *Table, oldPage []byte, key uint32, row []byte) {
	newPageNum := allocateNewPage(table.Pager)
	newPage := table.Pager.Pages[newPageNum]

	setNodeType(newPage, NODE_LEAF)
	setNodeRoot(newPage, false)
	setNumCells(newPage, 0)

	// Move upper half cells to new page
	numCells := getNumCells(oldPage)
	splitIndex := numCells / 2

	for i := splitIndex; i < numCells; i++ {
		existingKey := leafNodeGetKey(oldPage, i, table)
		existingRow := leafNodeGetValue(oldPage, i, table)
		leafNodeInsert(table, newPage, existingKey, existingRow)
	}

	setNumCells(oldPage, splitIndex)

	firstKeyOfNewPage := leafNodeGetKey(newPage, 0, table)
	if key < firstKeyOfNewPage {
		leafNodeInsert(table, oldPage, key, row)
	} else {
		leafNodeInsert(table, newPage, key, row)
	}

	// Find page number of oldPage
	var oldPageNum uint32
	for pNum, p := range table.Pager.Pages {
		if &p[0] == &oldPage[0] {
			oldPageNum = uint32(pNum)
			break
		}
	}

	if isNodeRoot(oldPage) {
		createNewRootAfterLeafSplit(table, oldPageNum, newPageNum, firstKeyOfNewPage)
	} else {
		fmt.Println("Leaf split handled (Parent internal node propagation needed).")
	}
}

func leafNodeInsertOrUpdate(table *Table, key uint32, row []byte) {
	// Load root page if not loaded
	if table.Pager.Pages[table.RootPageNum] == nil {
		page := make([]byte, PAGE_SIZE)
		n, err := table.Pager.File.ReadAt(page, int64(table.RootPageNum*PAGE_SIZE))

		// If file read failed or returned 0 bytes (brand new table), initialize fresh ROOT LEAF
		if err != nil || n == 0 {
			setNodeType(page, NODE_LEAF)
			setNodeRoot(page, true)
			setNumCells(page, 0)
		}
		table.Pager.Pages[table.RootPageNum] = page
	}

	leaf := findLeafNode(table, table.RootPageNum, key)
	numCells := getNumCells(leaf)
	_, found := leafNodeFind(table, leaf, key)

	if found {
		// Update existing row
		index, _ := leafNodeFind(table, leaf, key)
		cellOffset := leafNodeCellOffset(index, table)
		copy(pageCellData(leaf, cellOffset+LEAF_NODE_KEY_SIZE, table.RowSize), row)
	} else {
		// Max cells per leaf estimation based on page size
		maxCells := (PAGE_SIZE - LEAF_NODE_HEADER_SIZE) / leafNodeCellSize(table)
		if numCells >= maxCells {
			leafNodeSplitAndInsert(table, leaf, key, row)
		} else {
			leafNodeInsert(table, leaf, key, row)
		}
	}
}

func pageCellData(page []byte, offset uint32, size uint32) []byte {
	return page[offset : offset+size]
}

// ==========================================
// SERIALIZATION & PAGER MANAGEMENT
// ==========================================

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
				return nil, fmt.Errorf("invalid int value: %v", err)
			}
			offset := table.Columns[i].Offset
			putUint32(row, offset, uint32(value))
		case "string":
			if len(column) > int(table.Columns[i].Size) {
				return nil, fmt.Errorf("string exceeds maximum size")
			}
			offset := table.Columns[i].Offset
			copy(row[offset:], []byte(column))
		}
	}
	return row, nil
}

func DeserializeRow(table *Table, row []byte) ([]string, error) {
	columns := make([]string, len(table.Columns))
	for i, column := range table.Columns {
		offset := column.Offset
		switch column.Type {
		case "int":
			value := getUint32(row, offset)
			columns[i] = strconv.Itoa(int(value))
		case "string":
			strValue := string(row[offset : offset+column.Size])
			columns[i] = strings.TrimRight(strValue, "\x00")
		}
	}
	return columns, nil
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
	return &Pager{File: file}, uint32(fileInfo.Size()), nil
}

func (p *Pager) FlushPage(pageNum uint32, size uint32) error {
	if p.Pages[pageNum] == nil {
		return nil
	}
	_, err := p.File.WriteAt(p.Pages[pageNum], int64(pageNum*PAGE_SIZE))
	return err
}

func flushTable(table *Table) error {
	file, err := os.OpenFile(filepath.Join(DB_DIR, table.Name+".meta"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	return encoder.Encode(table)
}

func InitializeDatabase() *Database {
	db := &Database{Tables: make(map[string]*Table)}
	os.MkdirAll(DB_DIR, 0755)

	files, err := os.ReadDir(DB_DIR)
	if err != nil {
		return db
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".meta") {
			tableName := strings.TrimSuffix(file.Name(), ".meta")
			metaFile, err := os.Open(filepath.Join(DB_DIR, tableName+".meta"))
			if err != nil {
				continue
			}
			var table Table
			if json.NewDecoder(metaFile).Decode(&table) == nil {
				pager, _, err := NewPager(tableName)
				if err == nil {
					table.Pager = pager
					db.Tables[tableName] = &table
				}
			}
			metaFile.Close()
		}
	}
	return db
}

func CloseDatabase(db *Database) error {
	for _, table := range db.Tables {
		if table.Pager != nil {
			for pageNum, page := range table.Pager.Pages {
				if page != nil {
					_ = table.Pager.FlushPage(uint32(pageNum), PAGE_SIZE)
				}
			}
			_ = table.Pager.File.Close()
		}
		_ = flushTable(table)
	}
	return nil
}

// ==========================================
// STATEMENTS PARSERS & EXECUTORS
// ==========================================

func parseCreateStatement(db *Database, input string) *ParseError {
	input = strings.TrimPrefix(input, "create")
	tableName, columnStrings, found := strings.Cut(input, "with")
	if !found {
		return &ParseError{Message: "Missing 'with' keyword."}
	}
	columnStrings, primaryKey, found := strings.Cut(columnStrings, "primaryKey")
	if !found {
		return &ParseError{Message: "Missing 'primaryKey' keyword."}
	}
	tableName = strings.TrimSpace(tableName)
	primaryKey = strings.TrimSpace(primaryKey)

	pattern := regexp.MustCompile(`\(([a-zA-Z0-9_]+),\s*([a-zA-Z0-9_]+)\)`)
	matches := pattern.FindAllStringSubmatch(columnStrings, -1)
	var columns []Column
	var offset uint32 = 0
	for _, m := range matches {
		colType := strings.TrimSpace(m[2])
		var size uint32 = 4
		if colType == "string" {
			size = 255
		}
		columns = append(columns, Column{Name: strings.TrimSpace(m[1]), Type: colType, Size: size, Offset: offset})
		offset += size
	}

	pager, _, err := NewPager(tableName)
	if err != nil {
		return &ParseError{Message: err.Error()}
	}

	table := &Table{
		Name:        tableName,
		Columns:     columns,
		PrimaryKey:  primaryKey,
		RowSize:     offset,
		RootPageNum: 0,
		Pager:       pager,
	}

	db.Tables[tableName] = table
	fmt.Printf("Table '%s' created successfully.\n", tableName)
	return nil
}

func parseInsertStatement(db *Database, input string) *ParseError {
	input = strings.TrimPrefix(input, "insert")
	columnsString, tableName, found := strings.Cut(input, "into")
	if !found {
		return &ParseError{Message: "Missing 'into' keyword."}
	}
	tableName = strings.TrimSpace(tableName)
	table := db.Tables[tableName]
	if table == nil {
		return &ParseError{Message: "Table does not exist."}
	}

	columns := strings.Split(strings.TrimSpace(columnsString), ",")
	if len(columns) != len(table.Columns) {
		return &ParseError{Message: "Incorrect number of columns."}
	}

	for i := range columns {
		columns[i] = strings.TrimSpace(columns[i])
	}

	row, err := SerializeRow(table, columns)
	if err != nil {
		return &ParseError{Message: err.Error()}
	}

	// Extract primary key value (assumed to be the first int column or matching PrimaryKey)
	var primaryKeyValue uint32
	for i, col := range table.Columns {
		if col.Name == table.PrimaryKey {
			val, _ := strconv.Atoi(columns[i])
			primaryKeyValue = uint32(val)
			break
		}
	}

	leafNodeInsertOrUpdate(table, primaryKeyValue, row)
	fmt.Println("Row inserted successfully via B-Tree.")
	return nil
}

func parseSelectStatement(db *Database, input string) *ParseError {
	input = strings.TrimPrefix(input, "select")
	tableName := strings.TrimSpace(input)
	table := db.Tables[tableName]
	if table == nil {
		return &ParseError{Message: "Table does not exist."}
	}

	// Traverse to root or first loaded leaf to scan records
	if table.Pager.Pages[table.RootPageNum] == nil {
		page := make([]byte, PAGE_SIZE)
		_, err := table.Pager.File.ReadAt(page, int64(table.RootPageNum*PAGE_SIZE))
		if err != nil {
			fmt.Println("Table is empty.")
			return nil
		}
		table.Pager.Pages[table.RootPageNum] = page
	}

	leaf := findLeafNode(table, table.RootPageNum, 0)
	numCells := getNumCells(leaf)

	for i := uint32(0); i < numCells; i++ {
		rowData := leafNodeGetValue(leaf, i, table)
		cols, err := DeserializeRow(table, rowData)
		if err != nil {
			continue
		}
		fmt.Println("Row", i+1, ":", cols)
	}

	return nil
}

func metaCommand(db *Database, input string) {
	if input == ".exit" {
		_ = CloseDatabase(db)
		os.Exit(0)
	} else {
		fmt.Println("Unrecognized command.")
	}
}

func executeStatement(db *Database, input string) {
	if strings.HasPrefix(input, "create") {
		if err := parseCreateStatement(db, input); err != nil {
			fmt.Println("Error:", err.Error())
		}
	} else if strings.HasPrefix(input, "insert") {
		if err := parseInsertStatement(db, input); err != nil {
			fmt.Println("Error:", err.Error())
		}
	} else if strings.HasPrefix(input, "select") {
		if err := parseSelectStatement(db, input); err != nil {
			fmt.Println("Error:", err.Error())
		}
	} else {
		fmt.Println("Unrecognized keyword.")
	}
}

func main() {
	db := InitializeDatabase()
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	for scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(input, ".") {
			metaCommand(db, input)
		} else if len(input) > 0 {
			executeStatement(db, input)
		}
		fmt.Print("> ")
	}
}
