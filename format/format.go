package format

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/buildbuildio/pebbles/common"

	"github.com/vektah/gqlparser/v2/ast"
)

type Formatter interface {
	FormatSelectionSet(sets ast.SelectionSet)
}

func NewFormatter(w io.Writer) Formatter {
	return &formatter{
		indent: "\t",
		writer: w,
	}
}

type formatter struct {
	writer io.Writer

	indent      string
	indentSize  int
	emitBuiltin bool

	padNext  bool
	lineHead bool
}

func (f *formatter) writeString(s string) {
	_, _ = f.writer.Write([]byte(s))
}

func (f *formatter) writeIndent() *formatter {
	if f.lineHead {
		f.writeString(strings.Repeat(f.indent, f.indentSize))
	}
	f.lineHead = false
	f.padNext = false

	return f
}

func (f *formatter) WriteNewline() *formatter {
	f.writeString("\n")
	f.lineHead = true
	f.padNext = false

	return f
}

func (f *formatter) WriteWord(word string) *formatter {
	if f.lineHead {
		f.writeIndent()
	}
	if f.padNext {
		f.writeString(" ")
	}
	f.writeString(strings.TrimSpace(word))
	f.padNext = true

	return f
}

func (f *formatter) WriteString(s string) *formatter {
	if f.lineHead {
		f.writeIndent()
	}
	if f.padNext {
		f.writeString(" ")
	}
	f.writeString(s)
	f.padNext = false

	return f
}

func (f *formatter) IncrementIndent() {
	f.indentSize++
}

func (f *formatter) DecrementIndent() {
	f.indentSize--
}

func (f *formatter) NoPadding() *formatter {
	f.padNext = false

	return f
}

func (f *formatter) NeedPadding() *formatter {
	f.padNext = true

	return f
}

func (f *formatter) FormatDirectiveList(lists ast.DirectiveList) {
	if len(lists) == 0 {
		return
	}

	for _, dir := range lists {
		f.FormatDirective(dir)
	}
}

func (f *formatter) FormatDirective(dir *ast.Directive) {
	f.WriteString("@").WriteWord(dir.Name)
	f.FormatArgumentList(dir.Arguments)
}

func (f *formatter) FormatArgumentList(lists ast.ArgumentList) {
	if len(lists) == 0 {
		return
	}
	f.NoPadding().WriteString("(")
	for idx, arg := range lists {
		f.FormatArgument(arg)

		if idx != len(lists)-1 {
			f.NoPadding().WriteWord(",")
		}
	}
	f.WriteString(")").NeedPadding()
}

func (f *formatter) FormatArgument(arg *ast.Argument) {
	f.WriteWord(arg.Name).NoPadding().WriteString(":").NeedPadding()
	f.WriteString(arg.Value.String())
}

func (f *formatter) FormatSelectionSet(sets ast.SelectionSet) {
	if len(sets) == 0 {
		return
	}

	f.WriteString("{").WriteNewline()
	f.IncrementIndent()

	for _, sel := range sets {
		f.FormatSelection(sel)
	}

	f.DecrementIndent()
	f.WriteString("}")
}

func (f *formatter) FormatSelection(selection ast.Selection) {
	switch v := selection.(type) {
	case *ast.Field:
		f.FormatField(v)

	case *ast.FragmentSpread:
		f.FormatFragmentSpread(v)

	case *ast.InlineFragment:
		f.FormatInlineFragment(v)

	default:
		panic(fmt.Errorf("unknown Selection type: %T", selection))
	}

	f.WriteNewline()
}

func (f *formatter) FormatField(field *ast.Field) {
	if field.Alias != "" && field.Alias != field.Name {
		f.WriteWord(field.Alias).NoPadding().WriteString(":").NeedPadding()
	}
	f.WriteWord(field.Name)

	if len(field.Arguments) != 0 {
		f.NoPadding()
		f.FormatArgumentList(field.Arguments)
		f.NeedPadding()
	}

	f.FormatDirectiveList(field.Directives)

	f.FormatSelectionSet(field.SelectionSet)
}

func (f *formatter) FormatFragmentSpread(spread *ast.FragmentSpread) {
	f.WriteWord("...").WriteWord(spread.Name)

	f.FormatDirectiveList(spread.Directives)

	f.FormatSelectionSet(spread.Definition.SelectionSet)
}

func (f *formatter) FormatInlineFragment(inline *ast.InlineFragment) {
	f.WriteWord("...")
	if inline.TypeCondition != "" {
		f.WriteWord("on").WriteWord(inline.TypeCondition)
	}

	f.FormatDirectiveList(inline.Directives)

	f.FormatSelectionSet(inline.SelectionSet)
}

func DebugFormatSelectionSetWithArgs(s ast.SelectionSet) string {
	v := FormatSelectionSetWithArgs(s, nil)

	v = strings.ReplaceAll(v, "\t", " ")
	v = strings.ReplaceAll(v, "\n", "")
	space := regexp.MustCompile(`\s+`)
	v = space.ReplaceAllString(v, " ")

	return v
}

func FormatSelectionSetWithArgs(s ast.SelectionSet, operationName *string) string {
	buf := bytes.NewBufferString("")
	defer buf.Reset()
	f := NewFormatter(buf)
	f.FormatSelectionSet(s)
	v := buf.String()

	args := walkArgumentList(s)

	if len(args) == 0 {
		return v
	}

	var tuples []string
	for argName, argType := range args {
		tuples = append(tuples, fmt.Sprintf("$%s: %s", argName, argType))
	}
	// persistant order
	sort.Strings(tuples)

	argList := strings.Join(tuples, ", ")

	result := "query"
	if operationName != nil {
		result = fmt.Sprintf("%s %s", result, *operationName)
	}

	return fmt.Sprintf("%s (%s) %s", result, argList, v)
}

func walkArgumentList(s ast.SelectionSet) map[string]string {
	res := make(map[string]string)
	for _, f := range common.SelectionSetToFields(s, nil) {
		for _, a := range f.Arguments {
			if a.Value != nil && a.Value.Kind == ast.Variable && f.Definition != nil && f.Definition.Arguments != nil {
				ad := f.Definition.Arguments.ForName(a.Name)
				if ad == nil {
					continue
				}
				res[a.Value.Raw] = ad.Type.String()
			}
		}
		if f.SelectionSet != nil {
			stepRes := walkArgumentList(f.SelectionSet)
			for k, v := range stepRes {
				res[k] = v
			}
		}
	}

	return res
}
