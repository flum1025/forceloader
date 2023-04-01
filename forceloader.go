package forceloader

import (
	"flag"
	"fmt"
	"go/ast"
	"strings"

	"github.com/samber/lo"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const name = "forceloader"

var Analyzer = &analysis.Analyzer{
	Name: name,
	Doc:  "forceloader is testing tool for dataloader",
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
}

var (
	restrictedFieldSuffix *string
	ignoreResolvers       *string
)

//nolint: gochecknoinits
func init() {
	command := flag.NewFlagSet("forceloader", flag.ExitOnError)

	restrictedFieldSuffix = command.String("restrictedFieldSuffix", "UseCase", "")
	ignoreResolvers = command.String("ignoreResolvers", "queryResolver,mutationResolver", "")

	Analyzer.Flags = *command
}

var resolvers []string
var restrictedFields []string

func run(pass *analysis.Pass) (any, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return false, fmt.Errorf("failed to initialized")
	}

	resolvers = append(resolvers, parseResolvers(inspect)...)
	restrictedFields = append(restrictedFields, parseRestictedFields(inspect)...)
	ignoreResolvers := strings.Split(*ignoreResolvers, ",")
	ignores := extractIgnores(inspect)

	inspect.Preorder([]ast.Node{
		(*ast.FuncDecl)(nil),
	}, func(nd ast.Node) {
		n, ok := nd.(*ast.FuncDecl)
		if !ok || n.Recv == nil || len(n.Recv.List) == 0 {
			return
		}

		recv, ok := lo.Find(n.Recv.List, func(item *ast.Field) bool {
			if kind, ok := item.Type.(*ast.StarExpr); ok {
				if x, ok := kind.X.(*ast.Ident); ok {
					if lo.Contains(resolvers, x.Name) {
						return true
					}
				}
			}

			return false
		})

		if !ok || len(recv.Names) == 0 {
			return
		}

		recvName := recv.Names[0]

		detect(pass, n.Body.List, recvName, restrictedFields, ignoreResolvers, ignores)
	})

	return nil, nil
}

func extractIgnores(
	inspect *inspector.Inspector,
) []*ast.Comment {
	comments := []*ast.Comment{}

	inspect.Preorder([]ast.Node{
		(*ast.File)(nil),
	}, func(nd ast.Node) {
		file, ok := nd.(*ast.File)
		if !ok {
			return
		}

		cms := lo.FlatMap(file.Comments, func(group *ast.CommentGroup, _ int) []*ast.Comment {
			return lo.Filter(group.List, func(comment *ast.Comment, _ int) bool {
				if !strings.Contains(comment.Text, "nolint") {
					return false
				}

				rawComment := strings.TrimSpace(strings.Replace(comment.Text, "//", "", 1))
				parsedComment := lo.Map(strings.Split(rawComment, ":"), func(value string, _ int) string {
					return strings.TrimSpace(value)
				})

				if len(parsedComment) == 0 {
					return false
				}

				targets := lo.Map(strings.Split(parsedComment[1], ","), func(value string, _ int) string {
					return strings.TrimSpace(value)
				})

				return lo.Some(targets, []string{name})
			})
		})

		comments = append(comments, cms...)
	})

	return comments
}

func detect(
	pass *analysis.Pass,
	list []ast.Stmt,
	recvName *ast.Ident,
	restrictedFields []string,
	ignoreResolvers []string,
	ignores []*ast.Comment,
) {
	nodes := lo.FlatMap(list, func(item ast.Stmt, _ int) []extractCallExprResult {
		return extractCallExpr(pass, item, nil, nil)
	})

	lo.ForEach(nodes, func(result extractCallExprResult, _ int) {
		selector, ok := result.callExpr.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}

		selector2, ok := selector.X.(*ast.SelectorExpr)
		if !ok {
			return
		}

		ident, ok := selector2.X.(*ast.Ident)
		if !ok {
			return
		}

		if ident.Obj != recvName.Obj {
			return
		}

		field, ok := ident.Obj.Decl.(*ast.Field)
		if !ok {
			return
		}

		kind, ok := field.Type.(*ast.StarExpr)
		if !ok {
			return
		}

		def, ok := kind.X.(*ast.Ident)
		if !ok {
			return
		}

		if lo.Contains(restrictedFields, selector2.Sel.Name) && !lo.Contains(ignoreResolvers, def.Name) {
			isIgnored := lo.SomeBy(ignores, func(c *ast.Comment) bool {
				commentPos := pass.Fset.Position(c.Pos())

				if commentPos.Line == result.line {
					return true
				}

				if commentPos.Line == result.line-1 && commentPos.Column == result.column {
					return true
				}

				return false
			})

			if isIgnored {
				return
			}

			pass.Report(analysis.Diagnostic{
				Pos:     selector2.Pos(),
				End:     selector2.End(),
				Message: fmt.Sprintf("%s cannot be used in %s", selector2.Sel.Name, def.Name),
			})
		}
	})
}

type extractCallExprResult struct {
	callExpr *ast.CallExpr
	line     int
	column   int
}

func newExtractCallExprResult(
	pass *analysis.Pass,
	item ast.Node,
	call *ast.CallExpr,
	line *int,
	column *int,
) extractCallExprResult {
	_line, _column := parsePos(pass, item, line, column)

	return extractCallExprResult{
		callExpr: call,
		line:     _line,
		column:   _column,
	}
}

func parsePos(
	pass *analysis.Pass,
	item ast.Node,
	line *int,
	column *int,
) (int, int) {
	pos := pass.Fset.Position(item.Pos())

	_line := pos.Line
	_column := pos.Column

	if line != nil {
		_line = *line
	}

	if column != nil {
		_column = *column
	}

	return _line, _column
}

func extractCallExpr(
	pass *analysis.Pass,
	item ast.Node,
	line *int,
	column *int,
) []extractCallExprResult {
	switch item := item.(type) {
	case *ast.IfStmt:
		if item.Init != nil {
			assignstmt, ok := item.Init.(*ast.AssignStmt)
			if !ok {
				return nil
			}

			_line, _column := parsePos(pass, item, line, column)

			return extractCallExpr(pass, assignstmt, &_line, &_column)
		}

		cond, ok := item.Cond.(*ast.BinaryExpr)
		if !ok {
			return nil
		}

		call, ok := cond.X.(*ast.CallExpr)
		if !ok {
			return nil
		}

		_line, _column := parsePos(pass, item, line, column)
		return extractCallExpr(pass, call, &_line, &_column)
	case *ast.AssignStmt:
		return lo.FlatMap(item.Rhs, func(_item ast.Expr, _ int) []extractCallExprResult {
			call, ok := _item.(*ast.CallExpr)
			if !ok {
				return nil
			}

			_line, _column := parsePos(pass, item, line, column)
			return extractCallExpr(pass, call, &_line, &_column)
		})
	case *ast.ExprStmt:
		call, ok := item.X.(*ast.CallExpr)
		if !ok {
			return nil
		}

		_line, _column := parsePos(pass, item, line, column)
		return extractCallExpr(pass, call, &_line, &_column)
	case *ast.ParenExpr:
		x, ok := item.X.(*ast.FuncLit)
		if !ok {
			return nil
		}

		return lo.FlatMap(x.Body.List, func(_item ast.Stmt, _ int) []extractCallExprResult {
			return extractCallExpr(pass, _item, nil, nil)
		})
	case *ast.CallExpr:
		_, ok := item.Fun.(*ast.SelectorExpr)
		if !ok {
			_line, _column := parsePos(pass, item, line, column)

			return extractCallExpr(pass, item.Fun, &_line, &_column)
		}

		return []extractCallExprResult{
			newExtractCallExprResult(pass, item, item, line, column),
		}
	}

	return nil
}

func parseRestictedFields(
	inspect *inspector.Inspector,
) []string {
	fields := []string{}

	inspect.Preorder([]ast.Node{
		(*ast.TypeSpec)(nil),
	}, func(nd ast.Node) {
		n, ok := nd.(*ast.TypeSpec)
		if !ok {
			return
		}

		if n.Name == nil {
			return
		}

		if n.Name.Name != "Resolver" {
			return
		}

		kind, ok := n.Type.(*ast.StructType)
		if !ok {
			return
		}

		if kind.Fields == nil {
			return
		}

		if len(kind.Fields.List) == 0 {
			return
		}

		targets := lo.FilterMap(kind.Fields.List, func(item *ast.Field, _ int) (string, bool) {
			selector, ok := item.Type.(*ast.SelectorExpr)
			if !ok {
				return "", false
			}

			if selector.Sel == nil {
				return "", false
			}

			if !strings.HasSuffix(selector.Sel.Name, *restrictedFieldSuffix) {
				return "", false
			}

			if len(item.Names) == 0 {
				return "", false
			}

			return item.Names[0].Name, true
		})

		fields = append(fields, targets...)
	})

	return fields
}

func parseResolvers(
	inspect *inspector.Inspector,
) []string {
	resolvers := []string{}

	inspect.Preorder([]ast.Node{
		(*ast.Ident)(nil),
	}, func(nd ast.Node) {
		n, ok := nd.(*ast.Ident)
		if !ok || n.Obj == nil {
			return
		}

		decl, ok := n.Obj.Decl.(*ast.TypeSpec)
		if !ok || decl.Name == nil {
			return
		}

		if !strings.HasSuffix(decl.Name.Name, "Resolver") {
			return
		}

		kind, ok := decl.Type.(*ast.StructType)
		if !ok || kind.Fields == nil || len(kind.Fields.List) == 0 {
			return
		}

		if !hasResolver(kind) {
			return
		}

		resolvers = append(resolvers, decl.Name.Name)
	})

	return resolvers
}

func hasResolver(kind *ast.StructType) bool {
	return lo.SomeBy(kind.Fields.List, func(item *ast.Field) bool {
		if kind, ok := item.Type.(*ast.StarExpr); ok {
			if x, ok := kind.X.(*ast.Ident); ok {
				if x.Obj == nil && x.Name == "Resolver" {
					return true
				}
			}
		}

		return false
	})
}
