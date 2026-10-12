## Overview

This project works through the various database components: 
```
USER Interface --> Command Router --> Parser  
```

V1 of the Database has a limtied set of functionality: 

- support 3 operations: creating a table, inserting a row and printing all rows
- reside only in memory (no persistence to disk)
- types are restricted to int and str 


## Parsing

Syntax: 
- CREATE: `create tableName with [](colName, type) primaryKey colName` 
- INSERT: `insert []columns into tableName`
- SELECT: `select tableName`

Validations: 
- CREATE: primarykey should be a valid column
- INSERT: accept only complete rows, no partial input, column type should be valid ( str or int )

## Storage 

We obviously want to reduce random access, and replicate the data storage used in SSDs which implies that we need to store every row into slots in a page i.e. an array with varrying offsets. 

The tutorial sets the size of one page to 4,096 bytes (4KB). Why? Because your computer's operating system (Windows, Mac, Linux) naturally reads and writes to your hard drive in solid 4KB chunks. By matching this size, our database becomes incredibly fast.

Math: 
size (col_type ) = if int then 4 bytes else if string = 255 bytes

For every( col ), store offset, next_offset = offset + size( col_type ) 
row size = sum(size(cols))

at the time of insertion/reading: 
0. rowsPerPage = PAGE_SIZE / rowSize
1. pageNum =  rowNum / rowsPerPage
2. rowOffset = rowNum % rowPerPage 
3. byteOffset = rowOffset * rowSize ( because we write in bytes )
4. map the str/int to bytes and vice versa

```
Byte Coordinate:  0        1        2        3        4   5   6   7   8  ... 258
                ┌────────┬────────┬────────┬────────┬───┬───┬───┬───┬─────────┐
Data Array:     │  'a'   │  'p'   │  'p'   │  'l'   │'e'│ 0 │ 0 │ 0 │   ...   │ 
                └────────┴────────┴────────┴────────┴───┴───┴───┴───┴─────────┘
                └─── Mapped to "name" (First 255 bytes allocated) ────────────┘

Byte Coordinate:  259      260      261      262
                ┌────────┬────────┬────────┬────────┐
Data Array:     │  0x00  │  0x00  │  0x00  │  0x04  │
                └────────┴────────┴────────┴────────┘
                └─── Mapped to "cost" (Last 4 bytes) ┘

```


