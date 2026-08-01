# rotatewriter

`rotatewriter` is a Go package that provides an `io.Writer` with simple file rotation support.

## Features

- Drop-in `io.Writer` implementation
- Automatic log rotation when the file reaches a maximum size
- Configurable maximum number of backup files
- Optional automatic directory creation
- Safe handling of symlinks

## Installation

```bash
go get github.com/monitoring-forge/rotatewriter
```

## Usage

```go
package main

import (
    "log"

    "github.com/monitoring-forge/rotatewriter"
)

func main() {
    w, err := rotatewriter.New(
        rotatewriter.Filename("/var/log/myapp/app.log"),
        rotatewriter.MaxSize(100),      // Rotate when the file reaches 100 MiB
        rotatewriter.MaxBackups(7),     // Keep up to 7 backup files
        rotatewriter.AutoDirCreate(true),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer w.Close()

    logger := log.New(w, "", log.LstdFlags)
    logger.Println("application started")
}
```

## Options

| Option | Default | Description |
|--------|---------|-------------|
| `Filename` | required | Path to the log file. |
| `MaxSize` | 100 | Maximum size of the active log file in MiB. |
| `MaxBackups` | 7 | Maximum number of rotated backup files to retain. |
| `AutoDirCreate` | false | Automatically create the log directory if it does not exist. |

## Rotation behavior

- When a write would cause the active log file to exceed `MaxSize`, the current file is closed and renamed to `{basename}-{unixnano}{ext}`.
- Old backup files are removed to keep at most `MaxBackups` files.
- A new active log file is created at the original `Filename`.

## License

See [LICENSE](LICENSE).
