# Vocabulary Import Design

## Overview

Add a CLI command to import known vocabulary from Skritter TSV exports, preventing Polybius from creating flashcards for words the user has already learned.

## Command

```
polybius vocab import <file>
```

## Input Format

Skritter TSV export with tab-separated columns:
```
simplified	traditional	pinyin	definition
不过	不过	bu2guo4	but; however; only; cannot be more (intensifier for adjectives)
```

Only the simplified column is used. Traditional is ignored.

## Behavior

1. Read TSV file line by line
2. For each line:
   - Parse tab-separated values (expect 4 columns)
   - Extract simplified character (column 0), pinyin (column 2), definition (column 3)
   - Check if word already exists with status="known"
   - If already known: print `= 词 (already known)`, increment skip counter
   - If new: upsert with status="known", print `+ 词 (pinyin) - definition`, increment add counter
3. Print summary: `Imported X words (Y new, Z already known)`

## Output Example

```
$ polybius vocab import skritter-export.tsv

+ 不过 (bu2guo4) - but; however
+ 医生 (yi1sheng1) - doctor; physician
+ 多少 (duo1shao5) - how much; how many
= 我 (already known)
+ 电子邮件 (dian4zi3 you2jian4) - email
...

Imported 335 words (312 new, 23 already known)
```

## Error Handling

- **File not found**: Print error and exit
- **Malformed line**: Warn and skip: `! line 42: expected 4 columns, got 2 (skipped)`
- **Empty lines**: Silently skipped
- **Database error**: Print error and exit

Import continues past malformed lines.

## File Changes

| File | Change |
|------|--------|
| `brain/cmd/polybius/main.go` | Add `case "vocab":` routing to `runVocab()` |
| `brain/internal/vocab/import.go` | New file with `ImportTSV()` function |

## Data Flow

```
TSV File
    ↓
ImportTSV() parses lines
    ↓
storage.DB.IsWordKnown() checks existing status
    ↓
storage.DB.UpsertVocabulary() inserts/updates with status="known"
    ↓
Verbose output per word + summary
```
