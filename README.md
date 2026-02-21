# badger-cli

A terminal UI for inspecting BadgerDB keys and values quickly and safely.

Created by Savas Ayik. Short note and updates: https://savasayik.com

## Features

- Key list with search/filter
- Value viewer with `text`, `hex`, `base64`, and `json` formats
- Inline edit and save (Ctrl+S)
- Delete keys and delete by pattern
- Group counts by prefix
- About dialog (F1)

## Requirements

- Go 1.25+

## Build

```bash
go build -o badger-cli .
```

## Run

```bash
./badger-cli --dbpath ./data/badger

You can use --dbpath or -d flag. 
```

## Keybindings

- `↑/↓` move in list
- `Enter` load value and focus right panel
- `Esc`/`Shift+←` back to list (when right panel is focused)
- `t` `h` `b` `j` switch format (text/hex/base64/json)
- `/` filter keys
- `e` edit selected value
- `d` or `Delete` delete selected key
- `p` delete by pattern
- `g` group counts
- `F1` about me
- `q` quit

## Notes

- The tool reads values lazily and supports large databases.
- Editing respects the current format.
