## Overview

A lightweight database. 
The high level architecture is: 

USER Interface ──> Command Router ──> Parser ──> Storage Engine ──> Pager ──> Disk

### V1 Functionality & Constraints
* **Operations:** Supports 3 core operations: creating a table schema, inserting rows, and printing all rows.
* **Persistence:** Fully persistent. Table layouts and raw data blocks survive application restarts.
* **Type System:** Restricted strictly to `int` and `string` types.
* **Storage Isolation:** All physical asset resources are locked inside a sandboxed `db/` subdirectory.

---

## Parsing & Validation

### Syntax Rules
* **CREATE:** `create tableName with (colName, type) (colName, type) primaryKey colName`
* **INSERT:** `insert value1, value2 into tableName` (Strict left-to-right creation order)
* **SELECT:** `select from tableName`

### Validations
* **CREATE:** The specified `primaryKey` text token must match an active column name in the definition block. Table names must be unique; attempts to overwrite an existing table trigger an validation exception.
* **INSERT:** Accepts only complete rows (no partial inputs). Values must strictly match the data types defined during creation (`int` or double-quoted `"string"`).

---

## Storage Engine Architecture

To maximize sequential performance and eliminate arbitrary disk head movement, data storage mimics low-level SSD/HDD page blocks by packing rows tightly into slot boundaries inside 4,096-byte (4KB) pages. 

By matching the natural block size utilized by virtual memory systems across modern operating systems, the engine forces the OS to read and write entire pages as solid, un-fragmented memory blocks.

### Structural Layout Math
* **Data Sizes:** `int` = 4 bytes • `string` = 255 bytes (fixed buffer space).
* **Offsets:** For every column, `next_offset = current_offset + size(col_type)`. Total `rowSize = sum(sizes)`.

### Memory Slot Calculation
When reading or inserting records at a specific row sequence index (`rowNum`), the physical byte coordinates are resolved dynamically using metadata properties:
1. `rowsPerPage = PAGE_SIZE / rowSize`
2. `pageNum = rowNum / rowsPerPage`
3. `rowOffset = rowNum % rowsPerPage`
4. `byteOffset = rowOffset * rowSize`

#### Serialized Row Byte Mapping Example (`"apple", 4`):
```
Byte Coordinate:  0        1        2        3        4   5   6   7   8  ... 254
┌────────┬────────┬────────┬────────┬───┬───┬───┬───┬─────────┐
Data Array:     │  'a'   │  'p'   │  'p'   │  'l'   │'e'│ 0 │ 0 │ 0 │   ...   │
└────────┴────────┴────────┴────────┴───┴───┴───┴───┴─────────┘
└─── Mapped to "name" (First 255 bytes allocated) ────────────┘
Byte Coordinate:  255      256      257      258
┌────────┬────────┬────────┬────────┐
Data Array:     │  0x00  │  0x00  │  0x00  │  0x04  │
└────────┴────────┴────────┴────────┘
└─── Mapped to "cost" (Last 4 bytes) ┘
```

---

## The Pager Layer

The `Pager` struct serves as the **Memory Manager** and **I/O Controller** for our database files. Instead of letting the high-level storage engine talk directly to the hard drive, all file interactions flow through the Pager layer.
```

┌────────────────┐        Request Page X        ┌──────────────┐
│ Storage Engine │ ───────────────────────────> │    Pager     │
└────────────────┘                              └──────┬───────┘
▲                                                      │
│      If Cache Hit: Return RAM Array                  │ Check Cache
├──────────────────────────────────────────────────────┤
│                                                      ▼
│      If Cache Miss: Fetch 4KB from Disk              │ ReadAt()
└──────────────────────────────────────────────────────┴─> [tableName.db]
```

### Core Responsibilities of the Pager:

1. **Virtual Memory Cache:** The Pager maintains a fixed-size cache array of page buffers (`Pages[100][]byte`). When a row is requested, the Pager checks RAM first. If the page is already in memory (**Cache Hit**), it returns it instantly, avoiding an expensive hard drive operation.
2. **On-Demand Loading (Lazy Loading):** If a page is requested but has not been loaded into RAM yet (**Cache Miss**), the Pager calculates the exact byte position in the physical file, reads a single 4KB block using `ReadAt()`, and brings it into memory. This ensures our database maintains a tiny memory footprint regardless of database size.
3. **Memory Boundary Isolation:** The Pager completely isolates raw file pointers from our SQL parsing logic. The parsing engine simply asks for a logical slot index, and the Pager handles file offsets, read/write bounds, and OS buffer synchronization (`File.Sync()`).

---

## Disk Persistence

Database tables are stored inside the `db/` subdirectory using two separate file structures:

* **Table Schema Metadata (`tableName.meta`):** Struct properties, sizes, and layout boundaries are compiled into a flat JSON format. This file is parsed at engine startup to rebuild the in-memory structural blueprints before data ingestion.
* **Binary Data Pages (`tableName.db`):** Row bytes are arranged continuously in flat 4KB chunks. Active page buffers are held in a cached allocation array, loaded from the filesystem on-demand during a cache miss, and completely flushed to disk using synchronous writes when calling the `.exit` meta-command.

## Commentary on Data Storage 
- Storing rows as unsorted arrays, is easy in terms of implementation. 
- But reads mean we need to scan the entire table to find a specific row. 
- So, as we move to v2 of the implementation, we will switch our datastorage from Array to Btree


## B tree

How pages are stored with B trees: 
```
┌────────────────────────────────────────────────────────────────────────┐
│                        PAGE HEADER (8-16 Bytes)                        │
│  - Node Type (Leaf or Internal)                                        │
│  - Cell Count (Number of items currently stored)                       │
│  - Right Child / Parent Pointers                                       │
├────────────────────────────────────────────────────────────────────────┤
│                     CELL POINTER ARRAY (2-byte offsets)                │
│  [ Offset to Cell 1 ]  [ Offset to Cell 2 ]  [ Offset to Cell 3 ] ...  │
├────────────────────────────────────────────────────────────────────────┤
│                       <--- Free Space --->                             │
├────────────────────────────────────────────────────────────────────────┤
│                       CELL PAYLOADS (Growing Upward)                   │
│  - Cell 3: [ Key ] [ Row Data ]                                        │
│  - Cell 2: [ Key ] [ Row Data ]                                        │
│  - Cell 1: [ Key ] [ Row Data ]                                        │
└────────────────────────────────────────────────────────────────────────┘
```

We will be storing both data and index in B tree, essentially turning this into an Index Organized Table. 


