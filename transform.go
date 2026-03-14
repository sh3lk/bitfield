package main

import (
	"go/ast"
	"go/token"
)

// PackageInfo holds all bitfield struct information for a package.
type PackageInfo struct {
	Structs map[string]*StructInfo // struct name → info
}

// Pass1 scans all files in a package and collects bitfield struct information.
// It rewrites struct declarations in-place, replacing bitfield fields with storage units.
func Pass1(fset *token.FileSet, files []*ast.File) (*PackageInfo, error) {
	pkg := &PackageInfo{
		Structs: make(map[string]*StructInfo),
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
