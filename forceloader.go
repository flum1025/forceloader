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

var Analyzer = &analysis.Analyzer{
	Name: "forceloader",
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

		detect(pass, n.Body.List, recvName, restrictedFields, ignoreResolvers)
	})

	return nil, nil
}

func detect(
	pass *analysis.Pass,
	list []ast.Stmt,
	recvName *ast.Ident,
	restrictedFields []string,
	ignoreResolvers []string,
) {
	nodes := lo.FlatMap(list, func(item ast.Stmt, _ int) []*ast.CallExpr {
		return extractCallExpr(item)
	})

	lo.ForEach(nodes, func(call *ast.CallExpr, _ int) {
		selector, ok := call.Fun.(*ast.SelectorExpr)
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
			pass.Report(analysis.Diagnostic{
				Pos:     selector2.Pos(),
				End:     selector2.End(),
				Message: fmt.Sprintf("%s cannot be used in %s", selector2.Sel.Name, def.Name),
			})
		}
	})
}

func extractCallExpr(item ast.Stmt) []*ast.CallExpr {
	switch item := item.(type) {
	case *ast.IfStmt:
		if item.Init != nil {
			assignstmt, ok := item.Init.(*ast.AssignStmt)
			if !ok {
				return nil
			}

			return extractCallExpr(assignstmt)
		}

		cond, ok := item.Cond.(*ast.BinaryExpr)
		if !ok {
			return nil
		}

		call, ok := cond.X.(*ast.CallExpr)
		if !ok {
			return nil
		}

		return []*ast.CallExpr{call}
	case *ast.AssignStmt:
		return lo.FilterMap(item.Rhs, func(item ast.Expr, _ int) (*ast.CallExpr, bool) {
			call, ok := item.(*ast.CallExpr)
			if !ok {
				return nil, false
			}

			return call, true
		})
	case *ast.ExprStmt:
		call, ok := item.X.(*ast.CallExpr)
		if !ok {
			return nil
		}

		return []*ast.CallExpr{call}
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
