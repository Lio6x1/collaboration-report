// 教师工具：仅用 Go 语法树标出声明和语句边界，不写入学习者工程。
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Field struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Comment   string   `json:"comment"`
	Uses      []string `json:"uses"`
	Tag       string   `json:"tag,omitempty"`
	Forwarded []string `json:"forwarded,omitempty"`
}
type Fact struct {
	Start   int    `json:"start"`
	Kind    string `json:"kind"`
	Text    string `json:"text"`
	Outcome string `json:"outcome,omitempty"`
	Scope   string `json:"scope,omitempty"`
}
type Decl struct {
	Start     int      `json:"start"`
	End       int      `json:"end"`
	Symbols   []string `json:"symbols"`
	Kind      string   `json:"kind"`
	Signature string   `json:"signature"`
	Comment   string   `json:"comment"`
	Fields    []Field  `json:"fields"`
	Params    []Field  `json:"params"`
	Results   []Field  `json:"results"`
	Cuts      []int    `json:"cuts"`
	Facts     []Fact   `json:"facts"`
}
type File struct {
	Path    string            `json:"path"`
	Decls   []Decl            `json:"decls"`
	Imports map[string]string `json:"imports"`
}

func main() {
	var paths []string
	if err := json.NewDecoder(os.Stdin).Decode(&paths); err != nil {
		panic(err)
	}
	var output []File
	for _, path := range paths {
		raw, err := os.ReadFile(filepath.FromSlash(path))
		if err != nil {
			panic(err)
		}
		src := strings.TrimRight(strings.TrimPrefix(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\ufeff"), "\r\n\t ") + "\n"
		fs := token.NewFileSet()
		file, err := parser.ParseFile(fs, path, src, parser.ParseComments)
		if err != nil {
			panic(err)
		}
		offset := func(p token.Pos) int { return fs.Position(p).Offset }
		slice := func(node ast.Node) string { return src[offset(node.Pos()):offset(node.End())] }
		fields := func(list *ast.FieldList) []Field {
			var out []Field
			if list == nil {
				return out
			}
			for _, f := range list.List {
				comment := strings.TrimSpace(f.Doc.Text() + " " + f.Comment.Text())
				if len(f.Names) == 0 {
					out = append(out, Field{Name: "(匿名/未命名)", Type: slice(f.Type), Comment: comment})
				}
				for _, n := range f.Names {
					value := Field{Name: n.Name, Type: slice(f.Type), Comment: comment}
					if f.Tag != nil {
						value.Tag = f.Tag.Value
					}
					out = append(out, value)
				}
			}
			return out
		}
		embeddedFields := func(list *ast.FieldList) []Field {
			out := fields(list)
			for i := range out {
				if out[i].Name == "(匿名/未命名)" {
					out[i].Name = strings.TrimPrefix(out[i].Type, "*")
					out[i].Comment = "嵌入的类型/接口，组合其字段或方法集合。 " + out[i].Comment
				}
			}
			return out
		}
		item := File{Path: path, Imports: map[string]string{}}
		for _, spec := range file.Imports {
			value, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				panic(err)
			}
			name := filepath.Base(value)
			if spec.Name != nil {
				name = spec.Name.Name
			}
			item.Imports[name] = value
		}
		for _, node := range file.Decls {
			d := Decl{Start: offset(node.Pos()), End: offset(node.End())}
			switch n := node.(type) {
			case *ast.GenDecl:
				if n.Tok == token.IMPORT {
					continue
				}
				d.Kind = n.Tok.String()
				d.Comment = n.Doc.Text()
				if n.Doc != nil {
					d.Start = offset(n.Doc.Pos())
				}
				for _, spec := range n.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						d.Symbols = append(d.Symbols, s.Name.Name)
						if st, ok := s.Type.(*ast.StructType); ok {
							d.Fields = append(d.Fields, embeddedFields(st.Fields)...)
						}
						if it, ok := s.Type.(*ast.InterfaceType); ok {
							d.Fields = append(d.Fields, embeddedFields(it.Methods)...)
						}
						if ft, ok := s.Type.(*ast.FuncType); ok {
							d.Params = fields(ft.Params)
							d.Results = fields(ft.Results)
						}
					case *ast.ValueSpec:
						for _, name := range s.Names {
							d.Symbols = append(d.Symbols, name.Name)
						}
					}
				}
				d.Signature = strings.Split(slice(n), "\n")[0]
			case *ast.FuncDecl:
				d.Kind = "func"
				d.Comment = n.Doc.Text()
				if n.Doc != nil {
					d.Start = offset(n.Doc.Pos())
				}
				name := n.Name.Name
				if n.Recv != nil && len(n.Recv.List) > 0 {
					name = strings.TrimPrefix(slice(n.Recv.List[0].Type), "*") + "." + name
				}
				d.Symbols = []string{name}
				d.Params = fields(n.Type.Params)
				d.Results = fields(n.Type.Results)
				d.Signature = slice(n.Type)
				if n.Body != nil {
					// 说明依据真实 AST；参数按声明对象匹配，避免同名局部量与别的函数串线。
					compact := func(node ast.Node) string { return strings.TrimSpace(slice(node)) }
					add := func(values *[]string, value string) {
						for _, old := range *values {
							if old == value {
								return
							}
						}
						*values = append(*values, value)
					}
					params := map[*ast.Object]int{}
					index := 0
					for _, field := range n.Type.Params.List {
						for _, name := range field.Names {
							params[name.Obj] = index
							index++
						}
					}
					var stack []ast.Node
					ast.Inspect(n.Body, func(child ast.Node) bool {
						if child == nil {
							stack = stack[:len(stack)-1]
							return true
						}
						nested := false
						for _, ancestor := range stack {
							if _, ok := ancestor.(*ast.FuncLit); ok {
								nested = true
								break
							}
						}
						if ident, ok := child.(*ast.Ident); ok && ident.Obj != nil {
							if slot, found := params[ident.Obj]; found {
								for i := len(stack) - 1; i >= 0; i-- {
									parent := stack[i]
									switch parent.(type) {
									case *ast.CallExpr, *ast.BinaryExpr, *ast.UnaryExpr, *ast.SelectorExpr, *ast.ReturnStmt, *ast.KeyValueExpr, *ast.AssignStmt:
										add(&d.Params[slot].Uses, compact(parent))
										i = -1
									}
								}
							}
						}
						factsBefore := len(d.Facts)
						switch value := child.(type) {
						case *ast.IfStmt:
							fact := Fact{Start: offset(value.Pos()), Kind: "condition", Text: compact(value.Cond)}
							if len(value.Body.List) > 0 {
								first := value.Body.List[0]
								switch first.(type) {
								case *ast.ReturnStmt, *ast.BranchStmt:
									fact.Outcome = compact(first)
								}
							}
							d.Facts = append(d.Facts, fact)
						case *ast.ReturnStmt:
							d.Facts = append(d.Facts, Fact{Start: offset(value.Pos()), Kind: "return", Text: compact(value)})
							if nested {
								break
							}
							if len(value.Results) == 1 && len(d.Results) > 1 {
								if call, ok := value.Results[0].(*ast.CallExpr); ok {
									for slot := range d.Results {
										add(&d.Results[slot].Forwarded, compact(call))
									}
									break
								}
							}
							if len(value.Results) == 0 {
								for slot := range d.Results {
									if d.Results[slot].Name != "(匿名/未命名)" {
										add(&d.Results[slot].Uses, d.Results[slot].Name)
									}
								}
							}
							for slot, result := range value.Results {
								if slot < len(d.Results) {
									add(&d.Results[slot].Uses, compact(result))
								}
							}
						case *ast.DeferStmt:
							d.Facts = append(d.Facts, Fact{Start: offset(value.Pos()), Kind: "defer", Text: compact(value.Call)})
						case *ast.GoStmt:
							d.Facts = append(d.Facts, Fact{Start: offset(value.Pos()), Kind: "go", Text: compact(value.Call)})
						case *ast.CallExpr:
							name := compact(value.Fun)
							for _, suffix := range []string{".BeginTx", ".Commit", ".Rollback", ".ExecContext", ".QueryContext", ".QueryRowContext", ".Scan", ".Lock", ".Unlock", ".WithTimeout", ".WithCancel"} {
								if strings.HasSuffix(name, suffix) {
									d.Facts = append(d.Facts, Fact{Start: offset(value.Pos()), Kind: "call", Text: compact(value)})
									break
								}
							}
						}
						if nested {
							for index := factsBefore; index < len(d.Facts); index++ {
								d.Facts[index].Scope = "匿名函数"
							}
						}
						stack = append(stack, child)
						return true
					})
					d.Signature = strings.TrimSpace(src[offset(n.Pos()):offset(n.Body.Lbrace)])
					ast.Inspect(n.Body, func(child ast.Node) bool {
						stmt, ok := child.(ast.Stmt)
						if !ok {
							return true
						}
						pos := offset(stmt.Pos())
						line := strings.LastIndex(src[:pos], "\n") + 1
						if strings.TrimSpace(src[line:pos]) == "" {
							d.Cuts = append(d.Cuts, line)
						}
						return true
					})
				}
			default:
				panic(fmt.Sprintf("unsupported Go declaration %T", node))
			}
			item.Decls = append(item.Decls, d)
		}
		output = append(output, item)
	}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		panic(err)
	}
}
