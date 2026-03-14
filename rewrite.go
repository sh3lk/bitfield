package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// Pass2 transforms field accesses on bitfield structs into inline bit operations:
//   - h.Field (rvalue)       → FieldType((h._bfN >> offset) & mask)
//   - h.Field = v            → h._bfN = h._bfN&^(mask<<shift) | UnitType(v)&mask<<shift
//   - h.Field op= v          → h._bfN = h._bfN&^(mask<<shift) | UnitType(FieldType(h._bfN>>offset&mask) op v)&mask<<shift
//   - h.Field++              → h._bfN = h._bfN&^(mask<<shift) | UnitType(FieldType(h._bfN>>offset&mask)+1)&mask<<shift
//   - h.Field--              → h._bfN = h._bfN&^(mask<<shift) | UnitType(FieldType(h._bfN>>offset&mask)-1)&mask<<shift
//   - T{Field: v1, Field2: v2} → T{_bfN: UnitType(v1)&mask | (UnitType(v2)&mask)<<shift}
//   - &h.Field               → error
func Pass2(fset *token.FileSet, files []*ast.File, pkg *PackageInfo) error {
	if len(pkg.Structs) == 0 {
		return nil
	}

	for _, file := range files {
		if err := rewriteFile(fset, file, pkg); err != nil {
			return err
		}
	}
	return nil
}

func rewriteFile(fset *token.FileSet, file *ast.File, pkg *PackageInfo) error {
	// Build variable→struct type maps for each function.
	funcTypes := map[*ast.FuncDecl]map[string]string{}
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		funcTypes[fd] = buildVarTypes(fd, pkg)
	}

	var rewriteErr error
	var varTypes map[string]string

	rewriteNode(file, func(c *astCursor) bool {
		if rewriteErr != nil {
			return false
		}

		// Track which function scope we're in.
		if fd, ok := c.Node().(*ast.FuncDecl); ok {
			varTypes = funcTypes[fd]
		}

		switch n := c.Node().(type) {
		case *ast.UnaryExpr:
			// &h.Field → error
			if n.Op == token.AND {
				if sel, ok := n.X.(*ast.SelectorExpr); ok {
					if _, _, ok := findFieldForSel(sel, varTypes, pkg); ok {
						pos := fset.Position(n.Pos())
						rewriteErr = fmt.Errorf("%s:%d: cannot take address of bitfield %s",
							pos.Filename, pos.Line, sel.Sel.Name)
						return false
					}
				}
			}

		case *ast.AssignStmt:
			if err := rewriteAssign(fset, n, c, varTypes, pkg); err != nil {
				rewriteErr = err
				return false
			}

		case *ast.IncDecStmt:
			if err := rewriteIncDec(n, c, varTypes, pkg); err != nil {
				rewriteErr = err
				return false
			}

		case *ast.CompositeLit:
			if err := rewriteCompositeLit(fset, n, c, pkg); err != nil {
				rewriteErr = err
				return false
			}
		}

		return true
	}, func(c *astCursor) {
		if rewriteErr != nil {
			return
		}

		// Post-order: rewrite rvalue selector expressions to inline get.
		sel, ok := c.Node().(*ast.SelectorExpr)
		if !ok {
			return
		}
		unit, field, ok := findFieldForSel(sel, varTypes, pkg)
		if !ok {
			return
		}
		c.Replace(MakeGetExpr(sel.X, unit, field))
	})

	return rewriteErr
}

// findFieldForSel resolves a selector expression to a bitfield.
// It only matches when the receiver's type is known via varTypes and is a
// bitfield struct. This prevents false matches on regular structs that happen
// to share a field name with a bitfield (e.g. GraphTile.transitionCount vs
// NodeInfo.transitionCount).
func findFieldForSel(sel *ast.SelectorExpr, varTypes map[string]string, pkg *PackageInfo) (*StorageUnit, *PlacedField, bool) {
	if varTypes == nil {
		return nil, nil, false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return nil, nil, false
	}
	structName, ok := varTypes[id.Name]
	if !ok {
		return nil, nil, false
	}
	info, ok := pkg.Structs[structName]
	if !ok {
		return nil, nil, false
	}
	return FindFieldInStruct(sel.Sel.Name, info)
}

// buildVarTypes scans a function for variable→struct type mappings.
func buildVarTypes(fd *ast.FuncDecl, pkg *PackageInfo) map[string]string {
	m := make(map[string]string)

	// Method receiver.
	if fd.Recv != nil {
		for _, recv := range fd.Recv.List {
			if typeName := extractStructTypeName(recv.Type); typeName != "" {
				if _, ok := pkg.Structs[typeName]; ok {
					for _, name := range recv.Names {
						m[name.Name] = typeName
					}
				}
			}
		}
	}

	// Function parameters.
	if fd.Type.Params != nil {
		for _, param := range fd.Type.Params.List {
			if typeName := extractStructTypeName(param.Type); typeName != "" {
				if _, ok := pkg.Structs[typeName]; ok {
					for _, name := range param.Names {
						m[name.Name] = typeName
					}
				}
			}
		}
	}

	// Body: var declarations and short assignments.
	if fd.Body != nil {
		scanBlockForVarTypes(fd.Body, m, pkg)
	}

	return m
}

// extractStructTypeName extracts the type name from T, *T, etc.
func extractStructTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return extractStructTypeName(t.X)
	default:
		return ""
	}
}

// scanBlockForVarTypes recursively scans a block for variable type declarations.
func scanBlockForVarTypes(block *ast.BlockStmt, m map[string]string, pkg *PackageInfo) {
	for _, stmt := range block.List {
		switch s := stmt.(type) {
		case *ast.DeclStmt:
			gd, ok := s.Decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				typeName := extractStructTypeName(vs.Type)
				if typeName == "" {
					continue
				}
				if _, ok := pkg.Structs[typeName]; ok {
					for _, name := range vs.Names {
						m[name.Name] = typeName
					}
				}
			}
		case *ast.AssignStmt:
			if s.Tok == token.DEFINE && len(s.Rhs) == 1 {
				if cl, ok := s.Rhs[0].(*ast.CompositeLit); ok {
					typeName := typeNameFromExpr(cl.Type)
					if typeName != "" {
						if _, ok := pkg.Structs[typeName]; ok {
							for _, lhs := range s.Lhs {
								if id, ok := lhs.(*ast.Ident); ok {
									m[id.Name] = typeName
								}
							}
						}
					}
				}
			}
		case *ast.IfStmt:
			scanBlockForVarTypes(s.Body, m, pkg)
			if els, ok := s.Else.(*ast.BlockStmt); ok {
				scanBlockForVarTypes(els, m, pkg)
			}
		case *ast.ForStmt:
			scanBlockForVarTypes(s.Body, m, pkg)
		case *ast.RangeStmt:
			scanBlockForVarTypes(s.Body, m, pkg)
		case *ast.SwitchStmt:
			scanBlockForVarTypes(s.Body, m, pkg)
		case *ast.BlockStmt:
			scanBlockForVarTypes(s, m, pkg)
		}
	}
}

// rewriteAssign handles h.Field = v and h.Field op= v.
func rewriteAssign(fset *token.FileSet, n *ast.AssignStmt, c *astCursor, varTypes map[string]string, pkg *PackageInfo) error {
	if len(n.Lhs) != 1 || len(n.Rhs) != 1 {
		return nil
	}

	sel, ok := n.Lhs[0].(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	unit, field, ok := findFieldForSel(sel, varTypes, pkg)
	if !ok {
		return nil
	}

	rhs := n.Rhs[0]

	switch n.Tok {
	case token.ASSIGN:
		// h.Field = v → inline set
		if err := checkConstantOverflow(fset, rhs, field); err != nil {
			return err
		}
		c.Replace(MakeSetStmt(sel.X, unit, field, rhs))

	case token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN,
		token.QUO_ASSIGN, token.REM_ASSIGN,
		token.AND_ASSIGN, token.OR_ASSIGN, token.XOR_ASSIGN,
		token.SHL_ASSIGN, token.SHR_ASSIGN, token.AND_NOT_ASSIGN:
		// h.Field op= v → set(get() op v)
		op := compoundToOp(n.Tok)
		getter := MakeGetExpr(sel.X, unit, field)
		c.Replace(MakeSetStmt(sel.X, unit, field, &ast.BinaryExpr{X: getter, Op: op, Y: rhs}))

	default:
		// := or other — don't transform
	}

	return nil
}

// rewriteIncDec handles h.Field++ and h.Field--.
func rewriteIncDec(n *ast.IncDecStmt, c *astCursor, varTypes map[string]string, pkg *PackageInfo) error {
	sel, ok := n.X.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	unit, field, ok := findFieldForSel(sel, varTypes, pkg)
	if !ok {
		return nil
	}

	getter := MakeGetExpr(sel.X, unit, field)

	var op token.Token
	if n.Tok == token.INC {
		op = token.ADD
	} else {
		op = token.SUB
	}

	c.Replace(MakeSetStmt(sel.X, unit, field, &ast.BinaryExpr{
		X:  getter,
		Op: op,
		Y:  &ast.BasicLit{Kind: token.INT, Value: "1"},
	}))

	return nil
}

// rewriteCompositeLit rewrites T{Field: v} and T{v1, v2} by substituting
// bitfield elements with storage unit keys:
//
//	T{_bfN: uint8(v1)&mask1 | (uint8(v2)&mask2)<<shift2, RegularField: v3}
func rewriteCompositeLit(fset *token.FileSet, n *ast.CompositeLit, c *astCursor, pkg *PackageInfo) error {
	typeName := typeNameFromExpr(n.Type)
	if typeName == "" {
		return nil
	}
	info, ok := pkg.Structs[typeName]
	if !ok {
		return nil
	}
	if len(n.Elts) == 0 {
		return nil
	}

	// Separate bitfield and regular elements, grouping bitfields by unit.
	type bfElt struct {
		field *PlacedField
		val   ast.Expr
	}
	unitElts := map[int][]bfElt{}
	var regularElts []ast.Expr

	_, isKeyed := n.Elts[0].(*ast.KeyValueExpr)

	if isKeyed {
		for _, elt := range n.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			keyIdent, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			unit, field, isBf := FindFieldInStruct(keyIdent.Name, info)
			if isBf {
				if err := checkConstantOverflow(fset, kv.Value, field); err != nil {
					return err
				}
				unitElts[unit.Index] = append(unitElts[unit.Index], bfElt{field, kv.Value})
			} else {
				regularElts = append(regularElts, elt)
			}
		}
	} else {
		// Positional init: map values by index to info.Fields.
		for i, elt := range n.Elts {
			if i >= len(info.Fields) {
				break
			}
			fi := &info.Fields[i]
			if fi.IsBitField {
				unit, field, ok := FindFieldInStruct(fi.Name, info)
				if ok {
					if err := checkConstantOverflow(fset, elt, field); err != nil {
						return err
					}
					unitElts[unit.Index] = append(unitElts[unit.Index], bfElt{field, elt})
				}
			} else {
				// Convert to keyed element (positional won't work after field rewrite).
				regularElts = append(regularElts, &ast.KeyValueExpr{
					Key:   ast.NewIdent(fi.Name),
					Value: elt,
				})
			}
		}
	}

	if len(unitElts) == 0 {
		return nil
	}

	// Build new elements: regular fields + one KeyValue per storage unit.
	newElts := append([]ast.Expr{}, regularElts...)

	for _, unit := range info.Layout.Units {
		bfs, ok := unitElts[unit.Index]
		if !ok {
			continue
		}

		fields := make([]*PlacedField, len(bfs))
		vals := make([]ast.Expr, len(bfs))
		for i, bf := range bfs {
			fields[i] = bf.field
			vals[i] = bf.val
		}

		newElts = append(newElts, &ast.KeyValueExpr{
			Key:   ast.NewIdent(fmt.Sprintf("_bf%d", unit.Index)),
			Value: MakeUnitInitExpr(unit.Type, fields, vals),
		})
	}

	n.Elts = newElts
	return nil
}

// checkConstantOverflow checks if val is an integer literal that doesn't fit
// in the bitfield width. Returns an error if overflow detected, nil otherwise.
func checkConstantOverflow(fset *token.FileSet, val ast.Expr, field *PlacedField) error {
	// Handle negative: -X represented as UnaryExpr{Op: -, X: BasicLit}
	var lit *ast.BasicLit
	negative := false

	switch v := val.(type) {
	case *ast.BasicLit:
		if v.Kind != token.INT {
			return nil
		}
		lit = v
	case *ast.UnaryExpr:
		if v.Op == token.SUB {
			if bl, ok := v.X.(*ast.BasicLit); ok && bl.Kind == token.INT {
				lit = bl
				negative = true
			}
		}
	}

	if lit == nil {
		return nil
	}

	// Parse the integer value.
	n, ok := parseInt(lit.Value)
	if !ok {
		return nil
	}
	if negative {
		n = -n
	}

	// Check range based on signedness.
	if field.Signed {
		minVal := -(int64(1) << (field.Width - 1))
		maxVal := int64(1)<<(field.Width-1) - 1
		if n < minVal || n > maxVal {
			pos := fset.Position(val.Pos())
			return fmt.Errorf("%s:%d: constant %d overflows bitfield %s (signed %d-bit, range %d..%d)",
				pos.Filename, pos.Line, n, field.Name, field.Width, minVal, maxVal)
		}
	} else {
		maxVal := int64(1)<<field.Width - 1
		if n < 0 || n > maxVal {
			pos := fset.Position(val.Pos())
			return fmt.Errorf("%s:%d: constant %d overflows bitfield %s (unsigned %d-bit, range 0..%d)",
				pos.Filename, pos.Line, n, field.Name, field.Width, maxVal)
		}
	}

	return nil
}

// parseInt parses a Go integer literal (decimal, 0x, 0o, 0b, with underscores).
func parseInt(s string) (int64, bool) {
	// Remove underscores (Go allows 1_000_000).
	clean := strings.ReplaceAll(s, "_", "")
	val, err := strconv.ParseInt(clean, 0, 64)
	return val, err == nil
}

func typeNameFromExpr(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return typeExprToString(t)
	default:
		return ""
	}
}

func compoundToOp(tok token.Token) token.Token {
	switch tok {
	case token.ADD_ASSIGN:
		return token.ADD
	case token.SUB_ASSIGN:
		return token.SUB
	case token.MUL_ASSIGN:
		return token.MUL
	case token.QUO_ASSIGN:
		return token.QUO
	case token.REM_ASSIGN:
		return token.REM
	case token.AND_ASSIGN:
		return token.AND
	case token.OR_ASSIGN:
		return token.OR
	case token.XOR_ASSIGN:
		return token.XOR
	case token.SHL_ASSIGN:
		return token.SHL
	case token.SHR_ASSIGN:
		return token.SHR
	case token.AND_NOT_ASSIGN:
		return token.AND_NOT
	default:
		return token.ADD
	}
}

// --- AST cursor for in-place rewriting ---

type astCursor struct {
	parent   ast.Node
	name     string
	index    int
	node     ast.Node
	replaced bool
	newNode  ast.Node
}

func (c *astCursor) Node() ast.Node { return c.node }
func (c *astCursor) Replace(n ast.Node) {
	c.replaced = true
	c.newNode = n
}

// rewriteNode walks the AST calling pre before visiting children and post after.
// If pre returns false, children are not visited.
// The cursor allows replacing the current node in its parent.
func rewriteNode(root ast.Node, pre func(*astCursor) bool, post func(*astCursor)) {
	rewriteNodeImpl(root, nil, "", -1, pre, post)
}

func rewriteNodeImpl(node ast.Node, parent ast.Node, fieldName string, index int,
	pre func(*astCursor) bool, post func(*astCursor)) {

	if node == nil {
		return
	}

	c := &astCursor{parent: parent, name: fieldName, index: index, node: node}

	if pre != nil {
		if !pre(c) {
			return
		}
		if c.replaced {
			applyReplacement(parent, fieldName, index, c.newNode)
			// Don't walk children of the replacement — it's fully formed.
			return
		}
	}

	// Walk children based on node type.
	switch n := node.(type) {
	case *ast.File:
		for i, d := range n.Decls {
			rewriteNodeImpl(d, n, "Decls", i, pre, post)
		}
	case *ast.FuncDecl:
		if n.Body != nil {
			rewriteNodeImpl(n.Body, n, "Body", -1, pre, post)
		}
	case *ast.BlockStmt:
		for i := 0; i < len(n.List); i++ {
			rewriteNodeImpl(n.List[i], n, "List", i, pre, post)
		}
	case *ast.ExprStmt:
		rewriteNodeImpl(n.X, n, "X", -1, pre, post)
		if c2 := checkReplaced(n, "X", pre); c2 != nil {
			n.X = c2.(ast.Expr)
		}
	case *ast.AssignStmt:
		for i, x := range n.Lhs {
			rewriteNodeImpl(x, n, "Lhs", i, pre, post)
		}
		for i, x := range n.Rhs {
			rewriteNodeImpl(x, n, "Rhs", i, pre, post)
		}
	case *ast.ReturnStmt:
		for i, x := range n.Results {
			rewriteNodeImpl(x, n, "Results", i, pre, post)
		}
	case *ast.IfStmt:
		if n.Init != nil {
			rewriteNodeImpl(n.Init, n, "Init", -1, pre, post)
		}
		rewriteNodeImpl(n.Cond, n, "Cond", -1, pre, post)
		rewriteNodeImpl(n.Body, n, "Body", -1, pre, post)
		if n.Else != nil {
			rewriteNodeImpl(n.Else, n, "Else", -1, pre, post)
		}
	case *ast.ForStmt:
		if n.Init != nil {
			rewriteNodeImpl(n.Init, n, "Init", -1, pre, post)
		}
		if n.Cond != nil {
			rewriteNodeImpl(n.Cond, n, "Cond", -1, pre, post)
		}
		if n.Post != nil {
			rewriteNodeImpl(n.Post, n, "Post", -1, pre, post)
		}
		rewriteNodeImpl(n.Body, n, "Body", -1, pre, post)
	case *ast.RangeStmt:
		if n.Key != nil {
			rewriteNodeImpl(n.Key, n, "Key", -1, pre, post)
		}
		if n.Value != nil {
			rewriteNodeImpl(n.Value, n, "Value", -1, pre, post)
		}
		rewriteNodeImpl(n.X, n, "X", -1, pre, post)
		rewriteNodeImpl(n.Body, n, "Body", -1, pre, post)
	case *ast.SwitchStmt:
		if n.Init != nil {
			rewriteNodeImpl(n.Init, n, "Init", -1, pre, post)
		}
		if n.Tag != nil {
			rewriteNodeImpl(n.Tag, n, "Tag", -1, pre, post)
		}
		rewriteNodeImpl(n.Body, n, "Body", -1, pre, post)
	case *ast.CaseClause:
		for i, x := range n.List {
			rewriteNodeImpl(x, n, "List", i, pre, post)
		}
		for i, x := range n.Body {
			rewriteNodeImpl(x, n, "Body", i, pre, post)
		}
	case *ast.DeclStmt:
		rewriteNodeImpl(n.Decl, n, "Decl", -1, pre, post)
	case *ast.GenDecl:
		for i, s := range n.Specs {
			rewriteNodeImpl(s, n, "Specs", i, pre, post)
		}
	case *ast.ValueSpec:
		for i, v := range n.Values {
			rewriteNodeImpl(v, n, "Values", i, pre, post)
		}
	case *ast.IncDecStmt:
		rewriteNodeImpl(n.X, n, "X", -1, pre, post)
	case *ast.BinaryExpr:
		rewriteNodeImpl(n.X, n, "X", -1, pre, post)
		rewriteNodeImpl(n.Y, n, "Y", -1, pre, post)
	case *ast.UnaryExpr:
		rewriteNodeImpl(n.X, n, "X", -1, pre, post)
	case *ast.CallExpr:
		rewriteNodeImpl(n.Fun, n, "Fun", -1, pre, post)
		for i, a := range n.Args {
			rewriteNodeImpl(a, n, "Args", i, pre, post)
		}
	case *ast.SelectorExpr:
		rewriteNodeImpl(n.X, n, "X", -1, pre, post)
	case *ast.IndexExpr:
		rewriteNodeImpl(n.X, n, "X", -1, pre, post)
		rewriteNodeImpl(n.Index, n, "Index", -1, pre, post)
	case *ast.ParenExpr:
		rewriteNodeImpl(n.X, n, "X", -1, pre, post)
	case *ast.StarExpr:
		rewriteNodeImpl(n.X, n, "X", -1, pre, post)
	case *ast.CompositeLit:
		for i, e := range n.Elts {
			rewriteNodeImpl(e, n, "Elts", i, pre, post)
		}
	case *ast.KeyValueExpr:
		rewriteNodeImpl(n.Value, n, "Value", -1, pre, post)
	case *ast.SliceExpr:
		rewriteNodeImpl(n.X, n, "X", -1, pre, post)
		if n.Low != nil {
			rewriteNodeImpl(n.Low, n, "Low", -1, pre, post)
		}
		if n.High != nil {
			rewriteNodeImpl(n.High, n, "High", -1, pre, post)
		}
		if n.Max != nil {
			rewriteNodeImpl(n.Max, n, "Max", -1, pre, post)
		}
	case *ast.TypeAssertExpr:
		rewriteNodeImpl(n.X, n, "X", -1, pre, post)
	case *ast.FuncLit:
		rewriteNodeImpl(n.Body, n, "Body", -1, pre, post)
	}

	// Post-order callback.
	if post != nil {
		c.replaced = false
		c.node = node
		post(c)
		if c.replaced {
			applyReplacement(parent, fieldName, index, c.newNode)
		}
	}
}

func checkReplaced(_ ast.Node, _ string, _ func(*astCursor) bool) ast.Node {
	return nil
}

func applyReplacement(parent ast.Node, fieldName string, index int, newNode ast.Node) {
	if parent == nil {
		return
	}

	switch p := parent.(type) {
	case *ast.BlockStmt:
		if fieldName == "List" && index >= 0 {
			p.List[index] = newNode.(ast.Stmt)
		}
	case *ast.File:
		if fieldName == "Decls" && index >= 0 {
			p.Decls[index] = newNode.(ast.Decl)
		}
	case *ast.ExprStmt:
		if fieldName == "X" {
			p.X = newNode.(ast.Expr)
		}
	case *ast.AssignStmt:
		if fieldName == "Lhs" && index >= 0 {
			p.Lhs[index] = newNode.(ast.Expr)
		} else if fieldName == "Rhs" && index >= 0 {
			p.Rhs[index] = newNode.(ast.Expr)
		}
	case *ast.ReturnStmt:
		if fieldName == "Results" && index >= 0 {
			p.Results[index] = newNode.(ast.Expr)
		}
	case *ast.BinaryExpr:
		switch fieldName {
		case "X":
			p.X = newNode.(ast.Expr)
		case "Y":
			p.Y = newNode.(ast.Expr)
		}
	case *ast.UnaryExpr:
		if fieldName == "X" {
			p.X = newNode.(ast.Expr)
		}
	case *ast.CallExpr:
		if fieldName == "Fun" {
			p.Fun = newNode.(ast.Expr)
		} else if fieldName == "Args" && index >= 0 {
			p.Args[index] = newNode.(ast.Expr)
		}
	case *ast.SelectorExpr:
		if fieldName == "X" {
			p.X = newNode.(ast.Expr)
		}
	case *ast.IndexExpr:
		switch fieldName {
		case "X":
			p.X = newNode.(ast.Expr)
		case "Index":
			p.Index = newNode.(ast.Expr)
		}
	case *ast.ParenExpr:
		if fieldName == "X" {
			p.X = newNode.(ast.Expr)
		}
	case *ast.StarExpr:
		if fieldName == "X" {
			p.X = newNode.(ast.Expr)
		}
	case *ast.CompositeLit:
		if fieldName == "Elts" && index >= 0 {
			p.Elts[index] = newNode.(ast.Expr)
		}
	case *ast.KeyValueExpr:
		if fieldName == "Value" {
			p.Value = newNode.(ast.Expr)
		}
	case *ast.ValueSpec:
		if fieldName == "Values" && index >= 0 {
			p.Values[index] = newNode.(ast.Expr)
		}
	case *ast.IfStmt:
		switch fieldName {
		case "Init":
			p.Init = newNode.(ast.Stmt)
		case "Cond":
			p.Cond = newNode.(ast.Expr)
		case "Else":
			p.Else = newNode.(ast.Stmt)
		}
	case *ast.ForStmt:
		switch fieldName {
		case "Init":
			p.Init = newNode.(ast.Stmt)
		case "Cond":
			p.Cond = newNode.(ast.Expr)
		case "Post":
			p.Post = newNode.(ast.Stmt)
		}
	case *ast.SwitchStmt:
		switch fieldName {
		case "Tag":
			p.Tag = newNode.(ast.Expr)
		}
	case *ast.CaseClause:
		switch fieldName {
		case "List":
			if index >= 0 {
				p.List[index] = newNode.(ast.Expr)
			}
		case "Body":
			if index >= 0 {
				p.Body[index] = newNode.(ast.Stmt)
			}
		}
	case *ast.GenDecl:
		if fieldName == "Specs" && index >= 0 {
			p.Specs[index] = newNode.(ast.Spec)
		}
	case *ast.SliceExpr:
		switch fieldName {
		case "X":
			p.X = newNode.(ast.Expr)
		case "Low":
			p.Low = newNode.(ast.Expr)
		case "High":
			p.High = newNode.(ast.Expr)
		case "Max":
			p.Max = newNode.(ast.Expr)
		}
	case *ast.TypeAssertExpr:
		if fieldName == "X" {
			p.X = newNode.(ast.Expr)
		}
	}
}
