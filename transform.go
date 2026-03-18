package main

import (
	"go/ast"
	"go/token"
)

// PackageInfo holds all bitfield struct information for a package.
type PackageInfo struct {
	Structs      map[string]*StructInfo     // struct name → info
	Embeddings   map[string][]EmbeddingInfo // struct name → reachable bitfield structs via embedding
	DirectFields map[string]map[string]bool // struct name → set of directly declared field names (for shadowing)
}

// EmbeddingInfo describes how a bitfield struct is reachable through embedding.
type EmbeddingInfo struct {
	Path       []string // field access path, e.g. ["NodeInfo"] or ["B", "A"]
	StructName string   // the bitfield struct at the end of the path
}

// Pass1 scans all files in a package and collects bitfield struct information.
// It rewrites struct declarations in-place, replacing bitfield fields with storage units.
func Pass1(fset *token.FileSet, files []*ast.File) (*PackageInfo, error) {
	pkg := &PackageInfo{
		Structs:      make(map[string]*StructInfo),
		Embeddings:   make(map[string][]EmbeddingInfo),
		DirectFields: make(map[string]map[string]bool),
	}

	// First, collect all bitfield structs across all files.
	for _, file := range files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				info, err := ParseStructType(fset, ts.Name.Name, st)
				if err != nil {
					return nil, err
				}
				if info != nil {
					pkg.Structs[info.Name] = info
				}
			}
		}
	}

	if len(pkg.Structs) == 0 {
		return pkg, nil
	}

	// Scan all structs for embedding relationships and direct field names.
	directEmbeds := make(map[string][]string) // struct → directly embedded type names
	for _, file := range files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok || st.Fields == nil {
					continue
				}
				structName := ts.Name.Name
				fields := make(map[string]bool)
				for _, field := range st.Fields.List {
					if len(field.Names) == 0 {
						// Embedded field.
						embName := extractStructTypeName(field.Type)
						if embName != "" {
							directEmbeds[structName] = append(directEmbeds[structName], embName)
						}
					} else {
						for _, name := range field.Names {
							fields[name.Name] = true
						}
					}
				}
				if len(fields) > 0 {
					pkg.DirectFields[structName] = fields
				}
			}
		}
	}

	// Build transitive embedding map.
	pkg.Embeddings = buildEmbeddingMap(directEmbeds, pkg.Structs)

	// Rewrite struct declarations in-place.
	for _, file := range files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				info, exists := pkg.Structs[ts.Name.Name]
				if !exists {
					continue
				}
				// Rewrite the struct fields in-place, preserving line count.
				origFields := st.Fields
				newFields := GenerateStructFields(info)

				// Preserve brace positions.
				newFields.Opening = origFields.Opening
				newFields.Closing = origFields.Closing

				// Set each new field's position to the corresponding original field's line.
				for i := 0; i < len(newFields.List) && i < len(origFields.List); i++ {
					if len(newFields.List[i].Names) > 0 {
						newFields.List[i].Names[0].NamePos = origFields.List[i].Pos()
					}
				}

				// Add // comments at positions of removed fields to preserve line count.
				if len(origFields.List) > len(newFields.List) {
					for i := len(newFields.List); i < len(origFields.List); i++ {
						pos := origFields.List[i].Pos()
						if pos.IsValid() {
							file.Comments = append(file.Comments, &ast.CommentGroup{
								List: []*ast.Comment{{Slash: pos, Text: "//"}},
							})
						}
					}
				}

				st.Fields = newFields
			}
		}
	}

	return pkg, nil
}

// buildEmbeddingMap computes transitive embedding paths from any struct
// to reachable bitfield structs.
func buildEmbeddingMap(directEmbeds map[string][]string, structs map[string]*StructInfo) map[string][]EmbeddingInfo {
	result := make(map[string][]EmbeddingInfo)
	memo := make(map[string][]EmbeddingInfo)
	resolving := make(map[string]bool)

	var resolve func(typeName string) []EmbeddingInfo
	resolve = func(typeName string) []EmbeddingInfo {
		if infos, ok := memo[typeName]; ok {
			return infos
		}
		if resolving[typeName] {
			return nil
		}
		resolving[typeName] = true
		defer func() { delete(resolving, typeName) }()

		var infos []EmbeddingInfo
		for _, embName := range directEmbeds[typeName] {
			// Direct: embedded type is itself a bitfield struct.
			if _, ok := structs[embName]; ok {
				infos = append(infos, EmbeddingInfo{
					Path:       []string{embName},
					StructName: embName,
				})
			}
			// Transitive: through embedded type's own embeddings.
			for _, childInfo := range resolve(embName) {
				infos = append(infos, EmbeddingInfo{
					Path:       append([]string{embName}, childInfo.Path...),
					StructName: childInfo.StructName,
				})
			}
		}

		memo[typeName] = infos
		return infos
	}

	for typeName := range directEmbeds {
		infos := resolve(typeName)
		if len(infos) > 0 {
			result[typeName] = infos
		}
	}

	return result
}
