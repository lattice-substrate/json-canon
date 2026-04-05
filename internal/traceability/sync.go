// Package traceability provides tooling to regenerate and validate
// requirement-enforcement traceability artifacts from the canonical
// JSONL matrix and the live source tree.
package traceability

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	matrixCSVPath         = "REQ_ENFORCEMENT_MATRIX.csv"
	matrixJSONLPath       = "REQ_ENFORCEMENT_MATRIX.jsonl"
	matrixMarkdownPath    = "REQ_ENFORCEMENT_MATRIX.md"
	nolintInventoryPath   = "conformance/nolint_inventory.tsv"
	nolintInventoryHeader = "path\tline\tlinters\trequirement_ids\trationale\tdirective"
	matrixMarkdownPrefix  = `# Requirement Enforcement Matrix

Machine-readable mapping from requirement IDs to their enforcement artifacts.

## Format

` + "```" + `
requirement_id,domain,level,impl_file,impl_symbol,impl_line,test_file,test_function,gate
` + "```" + `

- **level**: L1 = unit test, L3 = conformance/integration test
- **gate**: TEST = ` + "`go test`" + `, CONFORMANCE = conformance harness

## Matrix

` + "```csv\n"
	matrixMarkdownSuffix = "```\n"
)

// MatrixRow represents a single row in the requirement enforcement matrix,
// mapping a requirement ID to its implementation and test artifacts.
type MatrixRow struct {
	RequirementID string `json:"requirement_id"`
	Domain        string `json:"domain"`
	Level         string `json:"level"`
	ImplFile      string `json:"impl_file"`
	ImplSymbol    string `json:"impl_symbol"`
	ImplLine      string `json:"impl_line"`
	TestFile      string `json:"test_file"`
	TestFunction  string `json:"test_function"`
	Gate          string `json:"gate"`
}

// NolintDirectiveRecord captures a single governed nolint directive
// extracted from the source tree for the nolint inventory artifact.
type NolintDirectiveRecord struct {
	Path           string
	Line           int
	Linters        string
	RequirementIDs string
	Rationale      string
	Directive      string
}

type symbolRange struct {
	start int
	end   int
}

// Sync regenerates traceability mirrors from the canonical JSONL matrix and live source tree.
func Sync(root string) error {
	rows, err := loadMatrixJSONLRows(filepath.Join(root, matrixJSONLPath))
	if err != nil {
		return err
	}
	if err = updateMatrixImplLines(root, rows); err != nil {
		return err
	}
	if err = validateMatrixTestFunctions(root, rows); err != nil {
		return err
	}
	if err = writeMatrixJSONL(filepath.Join(root, matrixJSONLPath), rows); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(root, matrixCSVPath), renderMatrixCSV(rows), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", matrixCSVPath, err)
	}
	if err = os.WriteFile(filepath.Join(root, matrixMarkdownPath), renderMatrixMarkdown(rows), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", matrixMarkdownPath, err)
	}
	records, err := CollectNolintInventory(root)
	if err != nil {
		return err
	}
	inventory, err := RenderNolintInventory(records)
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(root, nolintInventoryPath), inventory, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", nolintInventoryPath, err)
	}
	return nil
}

func loadMatrixJSONLRows(path string) ([]MatrixRow, error) {
	data, err := os.ReadFile(path) //nolint:gosec // REQ:TRACE-001 operator-controlled file paths in traceability sync tooling.
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	rows := make([]MatrixRow, 0, strings.Count(string(data), "\n"))
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row MatrixRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("parse %s line %d: %w", filepath.Base(path), i+1, err)
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s contains no rows", filepath.Base(path))
	}
	return rows, nil
}

func updateMatrixImplLines(root string, rows []MatrixRow) error {
	symbolsCache := map[string]map[string]symbolRange{}
	for i := range rows {
		row := &rows[i]
		if strings.TrimSpace(row.ImplFile) == "" || strings.TrimSpace(row.ImplSymbol) == "" {
			continue
		}
		path := filepath.Join(root, row.ImplFile)
		symbols, ok := symbolsCache[path]
		if !ok {
			var err error
			symbols, err = loadGoTopLevelSymbols(path)
			if err != nil {
				return err
			}
			symbolsCache[path] = symbols
		}
		loc, ok := symbols[row.ImplSymbol]
		if !ok {
			return fmt.Errorf("%s: impl_symbol %q not found", row.ImplFile, row.ImplSymbol)
		}
		row.ImplLine = strconv.Itoa(loc.start)
	}
	return nil
}

func validateMatrixTestFunctions(root string, rows []MatrixRow) error {
	funcsCache := map[string]map[string]struct{}{}
	for _, row := range rows {
		if strings.TrimSpace(row.TestFile) == "" || strings.TrimSpace(row.TestFunction) == "" {
			continue
		}
		path := filepath.Join(root, row.TestFile)
		funcs, ok := funcsCache[path]
		if !ok {
			var err error
			funcs, err = loadGoFunctionNames(path)
			if err != nil {
				return err
			}
			funcsCache[path] = funcs
		}
		baseFunc := row.TestFunction
		if idx := strings.IndexByte(baseFunc, '/'); idx >= 0 {
			baseFunc = baseFunc[:idx]
		}
		if _, ok := funcs[baseFunc]; !ok {
			return fmt.Errorf("%s: test_function %q not found", row.TestFile, baseFunc)
		}
	}
	return nil
}

func writeMatrixJSONL(path string, rows []MatrixRow) error {
	var b strings.Builder
	for _, row := range rows {
		data, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("marshal %s row %s: %w", filepath.Base(path), row.RequirementID, err)
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func renderMatrixCSV(rows []MatrixRow) []byte {
	var b strings.Builder
	b.WriteString("requirement_id,domain,level,impl_file,impl_symbol,impl_line,test_file,test_function,gate\n")
	for _, row := range rows {
		b.WriteString(strings.Join([]string{
			row.RequirementID,
			row.Domain,
			row.Level,
			row.ImplFile,
			row.ImplSymbol,
			row.ImplLine,
			row.TestFile,
			row.TestFunction,
			row.Gate,
		}, ","))
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func renderMatrixMarkdown(rows []MatrixRow) []byte {
	var b strings.Builder
	b.WriteString(matrixMarkdownPrefix)
	b.Write(renderMatrixCSV(rows))
	b.WriteString(matrixMarkdownSuffix)
	return []byte(b.String())
}

// CollectNolintInventory gathers the complete checked-in nolint inventory from the live source tree.
func CollectNolintInventory(root string) ([]NolintDirectiveRecord, error) {
	nolintRe := regexp.MustCompile(`^//\s*nolint:([a-z0-9]+(?:,[a-z0-9]+)*)\s+//\s*(.+)$`)
	reqIDRe := regexp.MustCompile(`[A-Z]+-[A-Z0-9]+-[0-9]+`)
	records := make([]NolintDirectiveRecord, 0, 32)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return skipInventoryDir(root, path)
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		fileRecords, err := collectNolintInventoryFromFile(root, path, nolintRe, reqIDRe)
		if err != nil {
			return err
		}
		records = append(records, fileRecords...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk source tree for nolint inventory: %w", err)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Path != records[j].Path {
			return records[i].Path < records[j].Path
		}
		return records[i].Line < records[j].Line
	})
	return records, nil
}

func skipInventoryDir(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err == nil {
		rel = filepath.ToSlash(rel)
		switch {
		case rel == "offline/runs":
			return filepath.SkipDir
		case strings.HasPrefix(rel, "offline/runs/"):
			return filepath.SkipDir
		}
	}
	switch filepath.Base(path) {
	case ".git", "vendor", ".tmp", ".extracted":
		return filepath.SkipDir
	default:
		return nil
	}
}

func collectNolintInventoryFromFile(root, path string, nolintRe, reqIDRe *regexp.Regexp) (records []NolintDirectiveRecord, retErr error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil, fmt.Errorf("resolve relative path %q: %w", path, err)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", rel, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close %s: %w", rel, closeErr))
		}
	}()
	sc := bufio.NewScanner(f)
	records = make([]NolintDirectiveRecord, 0, 4)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		record, ok, parseErr := parseNolintDirective(rel, lineNo, sc.Text(), nolintRe, reqIDRe)
		if parseErr != nil {
			return nil, parseErr
		}
		if ok {
			records = append(records, record)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", rel, err)
	}
	return records, nil
}

func parseNolintDirective(rel string, lineNo int, line string, nolintRe, reqIDRe *regexp.Regexp) (NolintDirectiveRecord, bool, error) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "//nolint") {
		return NolintDirectiveRecord{}, false, nil
	}
	if strings.Contains(trimmed, "nolint:all") {
		return NolintDirectiveRecord{}, false, fmt.Errorf("%s:%d uses forbidden blanket suppression: %s", rel, lineNo, trimmed)
	}
	match := nolintRe.FindStringSubmatch(trimmed)
	if match == nil {
		return NolintDirectiveRecord{}, false, fmt.Errorf("%s:%d must use //nolint:<linter>[,<linter>...] with inline rationale", rel, lineNo)
	}
	if strings.Contains(match[1], "all") {
		return NolintDirectiveRecord{}, false, fmt.Errorf("%s:%d must not suppress linter 'all'", rel, lineNo)
	}
	reqIDs := reqIDRe.FindAllString(match[2], -1)
	if len(reqIDs) == 0 {
		return NolintDirectiveRecord{}, false, fmt.Errorf("%s:%d nolint rationale must include at least one requirement ID", rel, lineNo)
	}
	return NolintDirectiveRecord{
		Path:           rel,
		Line:           lineNo,
		Linters:        match[1],
		RequirementIDs: strings.Join(uniqueSortedStrings(reqIDs), ","),
		Rationale:      match[2],
		Directive:      trimmed,
	}, true, nil
}

// RenderNolintInventory renders the governed nolint inventory artifact.
func RenderNolintInventory(records []NolintDirectiveRecord) ([]byte, error) {
	var b strings.Builder
	b.WriteString("# schema_version=nolint-inventory.v1\n")
	b.WriteString(nolintInventoryHeader)
	b.WriteByte('\n')
	for _, record := range records {
		fields := []string{
			record.Path,
			strconv.Itoa(record.Line),
			record.Linters,
			record.RequirementIDs,
			record.Rationale,
			record.Directive,
		}
		for _, field := range fields {
			if strings.ContainsAny(field, "\n\r\t") {
				return nil, fmt.Errorf("nolint inventory field contains invalid whitespace: %q", field)
			}
		}
		b.WriteString(strings.Join(fields, "\t"))
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func loadGoTopLevelSymbols(path string) (map[string]symbolRange, error) {
	data, err := os.ReadFile(path) //nolint:gosec // REQ:TRACE-001 operator-controlled file paths in traceability sync tooling.
	if err != nil {
		return nil, fmt.Errorf("read go file %s: %w", path, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, data, 0)
	if err != nil {
		return nil, fmt.Errorf("parse go file %s: %w", path, err)
	}
	symbols := make(map[string]symbolRange)
	declRange := func(n ast.Node) symbolRange {
		return symbolRange{
			start: fset.Position(n.Pos()).Line,
			end:   fset.Position(n.End()).Line,
		}
	}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			symbols[d.Name.Name] = declRange(d)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					symbols[s.Name.Name] = declRange(s)
				case *ast.ValueSpec:
					for _, name := range s.Names {
						symbols[name.Name] = declRange(s)
					}
				}
			}
		}
	}
	return symbols, nil
}

func loadGoFunctionNames(path string) (map[string]struct{}, error) {
	data, err := os.ReadFile(path) //nolint:gosec // REQ:TRACE-001 operator-controlled file paths in traceability sync tooling.
	if err != nil {
		return nil, fmt.Errorf("read go file %s: %w", path, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, data, 0)
	if err != nil {
		return nil, fmt.Errorf("parse go file %s: %w", path, err)
	}
	funcs := make(map[string]struct{})
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			funcs[fn.Name.Name] = struct{}{}
		}
	}
	return funcs, nil
}
