package orbi

import (
    "bufio"
    "os"
    "strings"
    "log"
)

// LoadDotEnv reads a .env-style file and sets environment variables for any
// keys that are not already set in the process environment. It's intentionally
// lightweight (no external dependency) and supports lines like `KEY=VALUE` and
// ignores comments starting with `#`.
func LoadDotEnv(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()

    scanner := bufio.NewScanner(f)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        // Split on first '='
        idx := strings.Index(line, "=")
        if idx <= 0 {
            continue
        }

        key := strings.TrimSpace(line[:idx])
        val := strings.TrimSpace(line[idx+1:])

        // Remove optional surrounding quotes
        if len(val) >= 2 {
            if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
                val = val[1 : len(val)-1]
            }
        }

        if key == "" {
            continue
        }

        if os.Getenv(key) == "" {
            if err := os.Setenv(key, val); err != nil {
                log.Printf("warning: failed to set env %s: %v", key, err)
            }
        }
    }

    return scanner.Err()
}
