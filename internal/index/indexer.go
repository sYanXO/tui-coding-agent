package index

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`      // "struct", "interface", "function", "method", "variable", "constant"
	Signature string `json:"signature"` // Function signature or type declaration
	Line      int    `json:"line"`
}

type FileInfo struct {
	Path    string    `json:"path"` // Relative path
	ModTime time.Time `json:"mod_time"`
	Symbols []Symbol  `json:"symbols,omitempty"`
}

type Index struct {
	Files map[string]*FileInfo `json:"files"`
}

type Indexer struct {
	workspace string
	indexPath string
	index     *Index
	mu        sync.RWMutex
}

func NewIndexer(workspace string) *Indexer {
	return &Indexer{
		workspace: workspace,
		indexPath: filepath.Join(workspace, ".agent_index.json"),
		index:     &Index{Files: make(map[string]*FileInfo)},
	}
}

// Load loads the index from disk if it exists.
func (idx *Indexer) Load() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	data, err := os.ReadFile(idx.indexPath)
	if err != nil {
		return err // file doesn't exist yet, which is fine
	}
	return json.Unmarshal(data, idx.index)
}

// Save writes the index to disk.
func (idx *Indexer) Save() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	data, err := json.MarshalIndent(idx.index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(idx.indexPath, data, 0644)
}

// Scan performs an incremental scan of the workspace.
func (idx *Indexer) Scan() error {
	_ = idx.Load()

	newFiles := make(map[string]*FileInfo)
	err := filepath.Walk(idx.workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(idx.workspace, path)
		if err != nil {
			return err
		}

		// Skip hidden dirs/files and common package manager / build cache folders
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "node_modules" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only parse Go files for symbol extraction in V1
		if !strings.HasSuffix(info.Name(), ".go") {
			return nil
		}

		idx.mu.RLock()
		existing, ok := idx.index.Files[relPath]
		idx.mu.RUnlock()

		// Incremental update check
		if ok && existing.ModTime.Equal(info.ModTime()) {
			newFiles[relPath] = existing
			return nil
		}

		// Parse the Go file
		symbols, err := parseGoFile(path)
		if err != nil {
			// If parsing fails, we skip/record empty symbols rather than failing the whole scan
			symbols = []Symbol{}
		}

		newFiles[relPath] = &FileInfo{
			Path:    relPath,
			ModTime: info.ModTime(),
			Symbols: symbols,
		}
		return nil
	})

	if err != nil {
		return err
	}

	idx.mu.Lock()
	idx.index.Files = newFiles
	idx.mu.Unlock()

	return idx.Save()
}

func parseGoFile(filePath string) ([]Symbol, error) {
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, fileBytes, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var symbols []Symbol

	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			kind := "function"
			if d.Recv != nil && len(d.Recv.List) > 0 {
				kind = "method"
			}

			// Capture signature from decl up to body
			start := fset.Position(d.Pos()).Offset
			end := fset.Position(d.End()).Offset
			if d.Body != nil {
				end = fset.Position(d.Body.Lbrace).Offset
			}
			sig := strings.TrimSpace(string(fileBytes[start:end]))
			sig = strings.ReplaceAll(sig, "\n", " ")
			// Clean up extra spaces
			sig = strings.Join(strings.Fields(sig), " ")

			symbols = append(symbols, Symbol{
				Name:      d.Name.Name,
				Kind:      kind,
				Signature: sig,
				Line:      fset.Position(d.Pos()).Line,
			})

		case *ast.GenDecl:
			if d.Tok == token.TYPE {
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					kind := "type"
					switch ts.Type.(type) {
					case *ast.StructType:
						kind = "struct"
					case *ast.InterfaceType:
						kind = "interface"
					}

					symbols = append(symbols, Symbol{
						Name:      ts.Name.Name,
						Kind:      kind,
						Signature: "type " + ts.Name.Name + " " + kind,
						Line:      fset.Position(ts.Pos()).Line,
					})
				}
			} else if d.Tok == token.CONST || d.Tok == token.VAR {
				kind := "variable"
				if d.Tok == token.CONST {
					kind = "constant"
				}
				for _, spec := range d.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, name := range vs.Names {
						symbols = append(symbols, Symbol{
							Name:      name.Name,
							Kind:      kind,
							Signature: kind + " " + name.Name,
							Line:      fset.Position(name.Pos()).Line,
						})
					}
				}
			}
		}
	}

	return symbols, nil
}

// Search queries the index for matching symbol names.
func (idx *Indexer) Search(query string) []map[string]any {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var results []map[string]any
	q := strings.ToLower(query)

	for relPath, file := range idx.index.Files {
		for _, sym := range file.Symbols {
			if strings.Contains(strings.ToLower(sym.Name), q) {
				results = append(results, map[string]any{
					"name":      sym.Name,
					"kind":      sym.Kind,
					"signature": sym.Signature,
					"file":      relPath,
					"line":      sym.Line,
				})
			}
		}
	}
	return results
}

// ListFile returns symbols in a single file.
func (idx *Indexer) ListFile(relPath string) []Symbol {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if file, ok := idx.index.Files[relPath]; ok {
		return file.Symbols
	}
	return nil
}

// GetRepoMap returns a text representation of the file list.
func (idx *Indexer) GetRepoMap() map[string]any {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	files := []string{}
	for relPath := range idx.index.Files {
		files = append(files, relPath)
	}

	return map[string]any{
		"files": files,
	}
}
