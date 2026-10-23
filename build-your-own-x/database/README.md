## Overview

A lightweight, persistent database engine written in Go. 
The high-level architecture flows as follows:

```

USER Interface ──> Command Router ──> Parser ──> Storage Engine ──> Pager ──> Disk
```

### V1 Functionality & Constraints
- **Operations:** Supports 3 core operations: creating a table schema, inserting rows, and querying all records.
- **Persistence:** Fully persistent. Table layouts and raw data blocks survive application restarts.
- **Type System:** Restricted strictly to `int` and fixed-size `string` types.
- **Storage Isolation:** All physical database assets are sandboxed inside a local `db/` subdirectory.

---

## Parsing & Validation

### Syntax Rules
- **CREATE:** `create tableName with (colName, type) (colName, type) primaryKey colName`
- **INSERT:** `insert value1, value2 into tableName` (Strict left-to-right creation order)
- **SELECT:** `select from tableName` (or `select tableName`)

### Validations
- **CREATE:** The specified `primaryKey` text token must match an active column name in the definition block. Table names must be unique; attempts to overwrite an existing table trigger a validation exception.
- **INSERT:** Accepts only complete rows (no partial inputs). Values must strictly match the data types defined during creation (`int` or double-quoted `"string"`).

---

## Storage Engine Architecture (Transitioning to v2 B-Tree)

To maximize performance and eliminate full table scans ($O(N)$), the database engine transitions from a flat sequential array storage model to a hierarchical **B+ Tree Index-Organized Table (IOT)**. 

Both index keys and row data payloads are packed directly into fixed-size 4,096-byte (4KB) disk and memory pages, mirroring OS virtual memory blocks to optimize I/O performance.

### Structural Layout Math
- **Data Sizes:** `int` = 4 bytes • `string` = 255 bytes (fixed buffer space).
- **Offsets:** For every column, `next_offset = current_offset + size(col_type)`. Total `rowSize = sum(sizes)`.

### Page Header Offsets (Common Header: 10 Bytes)
Every 4KB page begins with a self-describing header:


```

Byte Offset:   0       1             2               6              10
┌───────┬─────────────┬───────────────┬──────────────┬────────────────┐
Field:       │ Type  │ Is Root?    │ Parent Pointer│ Num Cells    │ (Free Space...)│
│(1 byte│ (1 byte)    │ (4 bytes)     │ (4 bytes)    │                │
└───────┴─────────────┴───────────────┴──────────────┴────────────────┘
▲       ▲             ▲               ▲
│       │             │               └─ NUM_CELLS_OFFSET = 6
│       │             └───────────────── POINTER_TO_PARENT_OFFSET = 2
│       └─────────────────────── NODE_IS_ROOT_OFFSET = 1
└─────────────────────────────────────── NODE_TYPE_OFFSET = 0

```

---

## B-Tree Node Types & Layouts

### 1. Leaf Nodes (`NODE_LEAF`)
Inside a Leaf Node, data is organized into **Cells**. Each cell contains a primary key combined with fixed-size row data:
$$\text{Cell Size} = \text{Key Size (4 bytes)} + \text{Row Size}$$

To find cell number `cellNum` on a page:
$$\text{Byte Offset} = \text{Header Size} + (\text{cellNum} \times \text{Cell Size})$$

#### Sorted Insertion Scenarios:
- **Scenario A (Append to End):** If the new key is larger than all existing keys, binary search returns an index equal to `numCells`. Cell shifting is skipped entirely ($O(1)$ placement).
- **Scenario B (Middle Insert):** If the key belongs in the middle, existing cells from the insertion index onward are shifted to the right using Go's `copy()` function to open a vacant slot.

#### Page Splitting:
When a leaf page reaches capacity:
1. A new page is allocated via the pager.
2. Existing cells are split evenly between the old page and the new page.
3. The dividing key is promoted upward to create or update a parent internal node. If the splitting node was the root, a new internal root node is created, increasing tree height.

---

### 2. Internal Nodes (`NODE_INTERNAL`)
Internal nodes hold **zero row data**. Their sole purpose is routing traffic during searches and insertions.

```

PAGE START (4096 bytes)
┌────────────────────────────────────────────────────────┐
│ COMMON HEADER (10 bytes)                               │
│  - Node Type (1 byte)                                  │
│  - Is Root? (1 byte)                                   │
│  - Parent Pointer (4 bytes)                            │
│  - Num Keys / Cells (4 bytes)                          │
├────────────────────────────────────────────────────────┤
│ INTERNAL BODY ARRAY (Repeating pairs)                  │
│  - Child Pointer 0 (4 bytes uint32 page number)        │
│  - Key 0           (4 bytes uint32 max key)            │
│  - Child Pointer 1 (4 bytes uint32 page number)        │
│  - Key 1           (4 bytes uint32 max key)            │
│  - ...                                                 │
│  - Rightmost Child Pointer (4 bytes uint32)            │
└────────────────────────────────────────────────────────┘

```

#### Routing Rule:
- If target key $\le$ `Key 0`, follow `Child Pointer 0`.
- If target key $\le$ `Key 1`, follow `Child Pointer 1`.
- If greater than all keys, follow the rightmost child pointer.

---

## The Pager Layer & Persistence


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

### Core Responsibilities:
1. **Virtual Memory Cache:** Maintains a fixed cache array (`Pages[100][]byte`) to serve frequently accessed pages instantly without disk overhead.
2. **On-Demand Loading (Lazy Loading):** Loads 4KB blocks from disk via `ReadAt()` only upon a cache miss.
3. **Disk Persistence:**
   - **Table Metadata (`tableName.meta`):** Stores JSON-encoded schema definitions, column layouts, and row sizes.
   - **Binary Data Pages (`tableName.db`):** Stores continuous 4KB node blocks flushed synchronously on application exit.

---

## Complete V2 Component Architecture


```

┌─────────────────────────────────────────────────────────────────┐
│                    TABLE METADATA (.meta)                       │
│  - Table Name, Columns, RowSize, RootPageNum                    │
├─────────────────────────────────────────────────────────────────┤
│                    PAGER & CACHE LAYER                          │
│  - Manages 4KB pages in memory & file I/O                       │
├─────────────────────────────────────────────────────────────────┤
│                    B-TREE ENGINE                                │
│  - findLeafNode()  ──> Traverses internal nodes down to leaf    │
│  - leafNodeInsert()──> Binary search & sorted cell insertion    │
│  - splitAndInsert()──> Handles page overflows & parent updates  │
└─────────────────────────────────────────────────────────────────┘

```

### Example

Output from a sample run: 
```
> create users with (id, int) (name, string) primaryKey id
Table 'users' created successfully.

> insert 10, "Alice" into users
Row inserted successfully via B-Tree.

> insert 5, "Bob" into users
Row inserted successfully via B-Tree.

> insert 20, "Charlie" into users
Row inserted successfully via B-Tree.

> insert 15, "Diana" into users
Row inserted successfully via B-Tree.

> select users
Row 1 : [5 "Bob"]
Row 2 : [10 "Alice"]
Row 3 : [15 "Diana"]
Row 4 : [20 "Charlie"]

> .exit
```