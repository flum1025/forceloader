package forceloader

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"strings"

	"github.com/samber/lo"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

const name = "forceloader"

var Analyzer = &analysis.Analyzer{
	Name: name,
	Doc:  "forceloader is testing tool for dataloader",
	Run:  run,
	Requires: []*analysis.Analyzer{
		buildssa.Analyzer,
	},
}

var (
	resolverStruct        *string
	restrictedPackages    *string
	ignoreResolverStructs *string
)

//nolint: gochecknoinits
func init() {
	command := flag.NewFlagSet("forceloader", flag.ExitOnError)

	resolverStruct = command.String("resolverStruct", "", "")
	restrictedPackages = command.String("restrictedPackages", "", "")
	ignoreResolverStructs = command.String("ignoreResolverStructs", "", "")

	Analyzer.Flags = *command
}

var (
	resolvers        []string
	restrictedFields []string
)

func run(pass *analysis.Pass) (any, error) {
	_ssa, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	if !ok {
		return false, fmt.Errorf("failed to initialized")
	}

	restrictedPackages := strings.Split(*restrictedPackages, ",")

	lo.ForEach(_ssa.SrcFuncs, func(fn *ssa.Function, _ int) {
		_isResolver := isResolver(fn)
		if !_isResolver {
			return
		}

		lo.ForEach(fn.Blocks, func(block *ssa.BasicBlock, _ int) {
			lo.ForEach(block.Instrs, func(inst ssa.Instruction, _ int) {
				call, ok := inst.(*ssa.Call)
				if !ok {
					return
				}

				named, ok := call.Call.Value.Type().(*types.Named)
				if !ok {
					return
				}

				isTarget := lo.Contains(restrictedPackages, named.Obj().Pkg().Path())
				if !isTarget {
					return
				}

				nodePos := pass.Fset.Position(call.Pos())

				astFile, err := parser.ParseFile(pass.Fset, nodePos.Filename, nil, parser.ParseComments)
				if err != nil {
					return
				}

				_isNolint := isNolint(pass, astFile, nodePos)
				if _isNolint {
					return
				}

				pass.Report(analysis.Diagnostic{
					Pos:     call.Call.Pos(),
					Message: fmt.Sprintf("%s cannot be used in %s", getSourceCaller(pass, call, astFile, nodePos), fn.String()),
				})
			})
		})
	})

	return nil, nil
}

func getSourceCaller(
	pass *analysis.Pass,
	call *ssa.Call,
	astFile *ast.File,
	nodePos token.Position,
) string {
	text := call.String()

	var prev ast.Node

	ast.Inspect(astFile, func(n ast.Node) bool {
		if n == nil {
			return true
		}

		if prev == nil {
			prev = n

			return true
		}

		startOffset := pass.Fset.Position(prev.Pos()).Offset
		endOffset := pass.Fset.Position(n.Pos()).Offset

		if startOffset <= nodePos.Offset && nodePos.Offset <= endOffset {
			ast.Inspect(prev, func(n ast.Node) bool {
				selector, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				var buf bytes.Buffer

				if err := printer.Fprint(&buf, token.NewFileSet(), selector); err != nil {
					panic(err)
				}

				text = buf.String()

				return false
			})

			return false
		}

		return true
	})

	return text
}

func isResolver(
	fn *ssa.Function,
) bool {
	ptrs := make([]*types.Pointer, 0, len(fn.FreeVars)+len(fn.Params))

	lo.ForEach(fn.FreeVars, func(param *ssa.FreeVar, _ int) {
		ptr, ok := param.Type().(*types.Pointer)
		if !ok {
			return
		}

		ptr2, ok := ptr.Elem().(*types.Pointer)
		if !ok {
			return
		}

		ptrs = append(ptrs, ptr2)
	})

	lo.ForEach(fn.Params, func(param *ssa.Parameter, _ int) {
		ptr, ok := param.Type().(*types.Pointer)
		if !ok {
			return
		}

		ptrs = append(ptrs, ptr)
	})

	return lo.SomeBy(ptrs, func(ptr *types.Pointer) bool {
		named, ok := ptr.Elem().(*types.Named)
		if !ok {
			return false
		}

		str, ok := named.Underlying().(*types.Struct)
		if !ok {
			return false
		}

		ignoreResolverStructs := strings.Split(*ignoreResolverStructs, ",")

		isIgnored := lo.Contains(ignoreResolverStructs, named.Obj().Type().String())
		if isIgnored {
			return false
		}

		vars := lo.Times(str.NumFields(), func(i int) *types.Var {
			return str.Field(i)
		})

		return lo.SomeBy(vars, func(v *types.Var) bool {
			if v.Embedded() {
				str := strings.Replace(v.Type().String(), "*", "", -1)

				if str == *resolverStruct {
					return true
				}
			}

			return false
		})
	})
}

func isNolint(
	pass *analysis.Pass,
	astFile *ast.File,
	nodePos token.Position,
) bool {
	comments := getCommentFromCall(pass, astFile, nodePos)

	return lo.SomeBy(comments, func(item string) bool {
		if !strings.Contains(item, "nolint") {
			return false
		}

		rawComment := strings.TrimSpace(strings.Replace(item, "//", "", 1))
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
}

func getCommentFromCall(
	pass *analysis.Pass,
	astFile *ast.File,
	nodePos token.Position,
) []string {
	return lo.FlatMap(astFile.Comments, func(comment *ast.CommentGroup, _ int) []string {
		commentPos := pass.Fset.Position(comment.Pos())

		if commentPos.Line == nodePos.Line {
			return lo.Map(comment.List, func(item *ast.Comment, _ int) string {
				return item.Text
			})
		}

		if commentPos.Line != nodePos.Line-1 {
			return nil
		}

		isCurrentComment := true

		ast.Inspect(astFile, func(n ast.Node) bool {
			if n == nil {
				return true
			}

			nPos := pass.Fset.Position(n.Pos())

			if nPos.Line == commentPos.Line {
				switch n.(type) {
				case *ast.Comment, *ast.CommentGroup:
				default:
					isCurrentComment = false

					return false
				}
			}

			return true
		})

		if isCurrentComment {
			return lo.Map(comment.List, func(item *ast.Comment, _ int) string {
				return item.Text
			})
		}

		return nil
	})
}
