package format

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/buildbuildio/pebbles/common"

	"github.com/vektah/gqlparser/v2/ast"
)

type BufferedFormatter struct {
	*Formatter
}

func NewBufferedFormatter() *BufferedFormatter {
	return &BufferedFormatter{
		Formatter: NewFormatter(),
	}
}

func NewDebugBufferedFormatter() *BufferedFormatter {
	return &BufferedFormatter{
		Formatter: NewFormatter().WithIndent("").WithNewLine(" "),
	}
}

func (f *BufferedFormatter) WithNewLine(newLine string) *BufferedFormatter {
	f.Formatter.WithNewLine(newLine)
	return f
}

func (f *BufferedFormatter) WithIndent(indent string) *BufferedFormatter {
	f.Formatter.WithIndent(indent)
	return f
}

func (f *BufferedFormatter) WithOperationName(operationName string) *BufferedFormatter {
	f.Formatter.WithOperationName(operationName)
	return f
}

func (f *BufferedFormatter) WithSchema(schema *ast.Schema) *BufferedFormatter {
	f.Formatter.WithSchema(schema)
	return f
}

func (f *BufferedFormatter) Copy() *BufferedFormatter {

	return &BufferedFormatter{
		Formatter: f.Formatter.Copy(),
	}
}

func (f *BufferedFormatter) FormatSelectionSet(sets ast.SelectionSet) string {
	buf := &bytes.Buffer{}
	defer buf.Reset()
	f.Formatter.WithWriter(buf).FormatSelectionSet(sets)

	return buf.String()
}

func NewFormatter() *Formatter {
	return &Formatter{
		indent:  "\t",
		newLine: "\n",
	}
}

type Formatter struct {
	writer io.Writer

	indent        string
	newLine       string
	indentSize    int
	operationName *string
	schema        *ast.Schema

	padNext  bool
	lineHead bool
}

func (f *Formatter) WithWriter(w io.Writer) *Formatter {
	f.writer = w
	return f
}

func (f *Formatter) WithNewLine(newLine string) *Formatter {
	f.newLine = newLine
	return f
}

func (f *Formatter) WithIndent(indent string) *Formatter {
	f.indent = indent
	return f
}

func (f *Formatter) WithOperationName(operationName string) *Formatter {
	f.operationName = &operationName
	return f
}

func (f *Formatter) WithSchema(schema *ast.Schema) *Formatter {
	f.schema = schema
	return f
}

func (f *Formatter) Copy() *Formatter {
	tmp := *f
	return &tmp
}

func (f *Formatter) write(s string) {
	_, _ = f.writer.Write([]byte(s))
}

func (f *Formatter) writeIndent() *Formatter {
	if f.lineHead {
		f.write(strings.Repeat(f.indent, f.indentSize))
	}
	f.lineHead = false
	f.padNext = false

	return f
}

func (f *Formatter) writeNewLine() *Formatter {
	f.write(f.newLine)
	f.lineHead = true
	f.padNext = false

	return f
}

func (f *Formatter) writeWord(word string) *Formatter {
	if f.lineHead {
		f.writeIndent()
	}
	if f.padNext {
		f.write(" ")
	}
	f.write(strings.TrimSpace(word))
	f.padNext = true

	return f
}

func (f *Formatter) writeString(s string) *Formatter {
	if f.lineHead {
		f.writeIndent()
	}
	if f.padNext {
		f.write(" ")
	}
	f.write(s)
	f.padNext = false

	return f
}

func (f *Formatter) incrementIndent() {
	f.indentSize++
}

func (f *Formatter) decrementIndent() {
	f.indentSize--
}

func (f *Formatter) noPadding() *Formatter {
	f.padNext = false

	return f
}

func (f *Formatter) needPadding() *Formatter {
	f.padNext = true

	return f
}

func (f *Formatter) formatDirectiveList(lists ast.DirectiveList) {
	if len(lists) == 0 {
		return
	}

	for _, dir := range lists {
		f.formatDirective(dir)
	}
}

func (f *Formatter) formatDirective(dir *ast.Directive) {
	f.writeString("@").writeWord(dir.Name)
	f.formatArgumentList(dir.Arguments)
}

func (f *Formatter) formatArgumentList(lists ast.ArgumentList) {
	if len(lists) == 0 {
		return
	}
	f.noPadding().writeString("(")
	for idx, arg := range lists {
		f.formatArgument(arg)

		if idx != len(lists)-1 {
			f.noPadding().writeWord(",")
		}
	}
	f.writeString(")").needPadding()
}

func (f *Formatter) formatArgument(arg *ast.Argument) {
	f.writeWord(arg.Name).noPadding().writeString(":").needPadding()
	f.writeString(arg.Value.String())
}

func (f *Formatter) walkArgumentList(s ast.SelectionSet) map[string]string {
	res := make(map[string]string)
	for _, field := range common.SelectionSetToFields(s, nil) {
		for _, a := range field.Arguments {
			if field.Definition == nil || field.Definition.Arguments == nil {
				break
			}

			if a.Value == nil {
				continue
			}

			ad := field.Definition.Arguments.ForName(a.Name)
			if ad == nil {
				continue
			}

			if len(a.Value.Children) > 0 && f.schema != nil {
				typeDef, ok := f.schema.Types[ad.Type.Name()]
				if !ok {
					continue
				}

				for k, v := range f.walkChildrenArgumentList(typeDef, a.Value.Children) {
					res[k] = v
				}
				continue
			}

			if a.Value.Kind == ast.Variable {
				res[a.Value.Raw] = ad.Type.String()
			}
		}
		if field.SelectionSet != nil {
			stepRes := f.walkArgumentList(field.SelectionSet)
			for k, v := range stepRes {
				res[k] = v
			}
		}
	}

	return res
}

func (f *Formatter) walkChildrenArgumentList(typeDef *ast.Definition, childs ast.ChildValueList) map[string]string {
	res := make(map[string]string)
	if f.schema == nil {
		return res
	}

	for _, ch := range childs {
		if ch.Value == nil {
			continue
		}

		if len(ch.Value.Children) > 0 && ch.Value.Definition != nil {
			chTypeDef, ok := f.schema.Types[ch.Value.Definition.Name]
			if !ok {
				continue
			}
			for k, v := range f.walkChildrenArgumentList(chTypeDef, ch.Value.Children) {
				res[k] = v
			}
			continue
		}

		if ch.Value.Kind == ast.Variable {
			ad := typeDef.Fields.ForName(ch.Name)
			if ad == nil {
				continue
			}
			res[ch.Value.Raw] = ad.Type.String()
		}
	}
	return res
}

func (f *Formatter) formatSelectionSet(sets ast.SelectionSet) {
	if len(sets) == 0 {
		return
	}

	f.writeString("{").writeNewLine()
	f.incrementIndent()

	for _, sel := range sets {
		f.formatSelection(sel)
	}

	f.decrementIndent()
	f.writeString("}")
}

func (f *Formatter) formatSelection(selection ast.Selection) {
	switch v := selection.(type) {
	case *ast.Field:
		f.formatField(v)

	case *ast.FragmentSpread:
		f.formatFragmentSpread(v)

	case *ast.InlineFragment:
		f.formatInlineFragment(v)

	default:
		panic(fmt.Errorf("unknown Selection type: %T", selection))
	}

	f.writeNewLine()
}

func (f *Formatter) formatField(field *ast.Field) {
	if field.Alias != "" && field.Alias != field.Name {
		f.writeWord(field.Alias).noPadding().writeString(":").needPadding()
	}
	f.writeWord(field.Name)

	if len(field.Arguments) != 0 {
		f.noPadding()
		f.formatArgumentList(field.Arguments)
		f.needPadding()
	}

	f.formatDirectiveList(field.Directives)

	f.formatSelectionSet(field.SelectionSet)
}

func (f *Formatter) formatFragmentSpread(spread *ast.FragmentSpread) {
	f.writeWord("...").writeWord(spread.Name)

	f.formatDirectiveList(spread.Directives)

	f.formatSelectionSet(spread.Definition.SelectionSet)
}

func (f *Formatter) formatInlineFragment(inline *ast.InlineFragment) {
	f.writeWord("...")
	if inline.TypeCondition != "" {
		f.writeWord("on").writeWord(inline.TypeCondition)
	}

	f.formatDirectiveList(inline.Directives)

	f.formatSelectionSet(inline.SelectionSet)
}

func (f *Formatter) FormatSelectionSet(sets ast.SelectionSet) {
	if len(sets) == 0 {
		return
	}

	args := f.walkArgumentList(sets)

	if len(args) == 0 {
		if f.operationName != nil {
			f.writeWord("query").writeWord(*f.operationName)
		}
	} else {
		var tuples []string
		for argName, argType := range args {
			tuples = append(tuples, fmt.Sprintf("$%s: %s", argName, argType))
		}
		// persistant order
		sort.Strings(tuples)

		argList := strings.Join(tuples, ", ")

		f.writeWord("query")

		if f.operationName != nil {
			f.writeString(*f.operationName)
		}

		f.writeWord("(" + argList + ")")
	}

	f.formatSelectionSet(sets)
}
