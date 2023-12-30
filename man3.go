package main

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/token"
)

func type_type(t *doc.Type) *ast.TypeSpec {
	return t.Decl.Specs[0].(*ast.TypeSpec)
}

func type_name(t *doc.Type) string {
	return type_type(t).Name.String()
}

func doPackage(m *M) {
	m.docs.Filter(ast.IsExported)
	if m.name == "" {
		m.name = m.pkg.Name
	}
	m.sec = "3"
	m.find_refs(nil) //need name and sec first so we can ignore self references

	m.do_header("Go Packages")
	m.do_name()

	//do synopsis
	m.section("SYNOPSIS")
	m.WriteString(".B import ")
	if *import_path != "" {
		m.WriteString(m.name)
		m.WriteByte(' ')
	}
	m.WriteString("\\*(lq")
	if *import_path != "" {
		m.WriteString(*import_path)
	} else {
		m.WriteString(m.name)
	}
	m.WriteString("\\(rq\n.sp")

	//build TOC
	if len(m.docs.Consts) > 0 { //TODO see bug above
		m.WriteString("\n.B Constants\n.sp 0")
	}
	if len(m.docs.Vars) > 0 { //TODO see bug above
		m.WriteString("\n.B Variables\n.sp 0")
	}
	for _, f := range m.docs.Funcs {
		m.WriteString("\n.RB \"func \" ")
		m.WriteString(f.Name)
		m.WriteString("\n.sp 0")
	}
	for _, t := range m.docs.Types {
		m.WriteString("\n.RB \"type \" ")
		m.WriteString(type_name(t))
		m.WriteString("\n.sp 0")
		ind := len(t.Funcs) > 0 || len(t.Methods) > 0
		if ind {
			m.WriteString("\n.RS")
		}
		for _, f := range t.Funcs {
			if !ast.IsExported(f.Name) {
				continue
			}
			m.WriteString("\n.RB \"func \" ")
			m.WriteString(f.Name)
			m.WriteString("\n.sp 0")
		}
		for _, mt := range t.Methods {
			if !ast.IsExported(mt.Name) {
				continue
			}
			m.WriteString("\n.RB \"func (")
			m.WriteString(mt.Recv)
			m.WriteString(") \" ")
			m.WriteString(mt.Name)
			m.WriteString("\n.sp 0")
		}
		if ind {
			m.WriteString("\n.RE")
		}
	}

	m.do_description()
	m.user_sections("DIAGNOSTICS", "ENVIRONMENT", "FILES")
	m.remaining_user_sections()

	if len(m.docs.Consts) > 0 {
		m.section("CONSTANTS")
		Values(m, m.docs.Consts)
	}

	if len(m.docs.Vars) > 0 {
		m.section("VARIABLES")
		Values(m, m.docs.Vars)
	}

	if len(m.docs.Funcs) > 0 {
		m.section("FUNCTIONS")
		Funcs(m.F, m.docs.Funcs)
	}

	if len(m.docs.Types) > 0 {
		m.section("TYPES")
	}
	for _, t := range m.docs.Types {
		name := type_name(t)
		m.nl()
		m.WriteString(".SS \"")
		m.WriteString(name)
		m.WriteString("\"\n.B type ")
		m.WriteString(name)
		m.WriteByte(' ')
		composite, unexported := false, false
		kind := "fields."
		switch typ := type_type(t).Type.(type) {
		case *ast.InterfaceType:
			m.WriteString("interface {\n.RS")
			unexported = methods(m.F, typ.Methods, false)
			composite = true
			kind = "methods."
		case *ast.StructType:
			m.WriteString("struct {\n.RS\n")
			unexported = fields(m.F, typ.Fields, "\n")
			composite = true
		default:
			m.WriteString(typesigs(type_type(t).Type))
		}
		if composite {
			m.nl()
			if unexported {
				m.WriteString(".sp 0\n.B //contains unexported ")
				m.WriteString(kind)
				m.WriteByte('\n')
			}
			m.WriteString(".RE\n.B }")
		}
		l := len(t.Doc) + len(t.Consts) + len(t.Vars) + len(t.Funcs)
		l += len(t.Methods)
		if l > 0 {
			m.PP()

			genDoc(m.F, t.Doc)

			Values(m, t.Consts)
			Values(m, t.Vars)

			Funcs(m.F, t.Funcs)
			Funcs(m.F, t.Methods)
		}
	}

	m.do_bugs()
	m.do_see_also()
	m.do_endmatter()
}

func genDoc(m *F, s string) {
	if len(s) == 0 {
		return
	}
	m.paras(unstring([]byte(s)))
}

//BUG(jmf): Does not render RHS of consts or vars for section 3.

func Values(m *M, V []*doc.Value) {
	for i, v := range V {
		genDoc(m.F, v.Doc)
		m.PP()
		m.WriteString(".B ")
		d := v.Decl
		switch d.Tok {
		case token.CONST:
			m.WriteString("const ")
		case token.VAR:
			m.WriteString("var ")
		}
		multiple := len(d.Specs) != 1
		if multiple {
			m.WriteString("(\n.RS")
		}
		for _, sp := range d.Specs {
			m.nl()
			m.WriteString(".B ")
			vs := sp.(*ast.ValueSpec)
			for k, n := range vs.Names {
				if !ast.IsExported(n.Name) {
					continue
				}
				m.WriteString(n.Name)
				if k != len(vs.Names)-1 {
					m.WriteString(", ")
				} else {
					m.WriteByte(' ')
				}
			}
			typesig(m, vs.Type) //okay if nil
			m.nl()
			m.WriteString(".sp 0\n")
			//TODO if vs.Type==nil can sometimes deduce type from vs.Values
			//TODO print values when possible
		}
		if multiple {
			m.WriteString(".RE\n.B )")
		}
		if i != len(V)-1 {
			m.WriteString("\n.sp 0\n")
		}
	}
}

func Funcs(m *F, F []*doc.Func) {
	for _, f := range F {
		if !ast.IsExported(f.Name) {
			continue
		}
		m.PP()
		m.BR.B("func ")
		if f.Recv != "" {
			m.BR.B("(")
			m.BR.B(f.Recv)
			m.BR.B(") ")
		}
		m.BR.B(f.Name)
		Func(m, f.Decl.Type, true)
		m.br()
		if len(f.Doc) > 0 {
			m.PP()
			genDoc(m, f.Doc)
		}
	}
}

func writer(m *F, br bool) func(string) {
	if br {
		return func(s string) { m.BR.B(s) }
	}
	return func(s string) { m.WriteString(s) }
}

func Func(m *F, f *ast.FuncType, decl bool) {
	str := writer(m, decl)
	str("(")
	params(m, f.Params.List, decl)
	str(")")
	if r := f.Results; r != nil {
		str(" ")
		p := len(r.List) > 1 || len(r.List[0].Names) > 0
		if p {
			str("(")
		}
		params(m, r.List, decl)
		if p {
			str(")")
		}
	}
}

func params(m *F, fl []*ast.Field, decl bool) {
	if len(fl) == 0 {
		return
	}
	str := writer(m, decl)
	for i, f := range fl {
		for j, n := range f.Names {
			if decl {
				m.BR.R(n.Name)
			} else {
				str(n.Name)
			}
			if j != len(f.Names)-1 {
				str(",")
			}
			str(" ")
		}
		str(typesigs(f.Type))
		if i != len(fl)-1 {
			str(", ")
		}
	}
}

func unwrapTypeName(v ast.Expr) *ast.Ident {
	switch v := v.(type) {
	case *ast.StarExpr:
		return unwrapTypeName(v.X)
	case *ast.SelectorExpr:
		return unwrapTypeName(v.X)
	case *ast.Ident:
		return v
	default:
		panic(fmt.Errorf("unknown expression in field type name %T: %#v", v, v))
	}
}

func fields(mr *F, fl *ast.FieldList, sep string) (unex bool) {
	if fl == nil || len(fl.List) == 0 {
		return
	}
	if sep != "\n" {
		sep += " "
	}
	for i, f := range fl.List {
		m := Formatter()
		uxc := 0
		for j, n := range f.Names {
			if !ast.IsExported(n.Name) {
				unex = true
				uxc++
				continue
			}
			m.WriteString(n.Name)
			if j != len(f.Names)-1 {
				m.WriteString(", ")
			} else {
				m.WriteByte(' ')
			}
		}
		if uxc == len(f.Names) && uxc != 0 {
			continue
		} else if len(f.Names) == 0 {
			v := unwrapTypeName(f.Type)
			if !ast.IsExported(v.Name) {
				continue
			}
		}
		sig := typesigs(f.Type)
		if sep != "\n" {
			m.BR.B(sig)
		} else {
			m.WriteString(sig)
		}
		if i != len(fl.List)-1 {
			if sep == "\n" {
				m.nl()
				m.WriteString(".sp 0\n")
			} else {
				m.WriteString(sep)
			}
		}
		if m.Len() > 0 {
			if sep == "\n" {
				mr.WriteString(".B ")
			}
			mr.Write(m.Bytes())
		}

	}
	return
}

func methods(m *F, fl *ast.FieldList, inline bool) (unex bool) {
	if fl == nil || len(fl.List) == 0 {
		return
	}
	for _, f := range fl.List {
		if !inline {
			m.nl()
			m.WriteString(".B ")
		}
		if f.Names != nil {
			name := f.Names[0].Name
			if ast.IsExported(name) {
				m.WriteString(name)
				Func(m, f.Type.(*ast.FuncType), false)
			} else {
				unex = true
			}
		} else {
			m.WriteString(typesigs(f.Type))
		}
		if inline {
			m.WriteString("; ")
		} else {
			m.nl()
			m.WriteString(".sp 0\n")
		}
	}
	return
}

func typesig(m *M, e interface{}) {
	b := Formatter()
	typesigi(b, e, false)
	m.Write(b.Bytes())
}

func typesigs(e interface{}) string {
	m := Formatter()
	typesigi(m, e, true)
	return string(m.Bytes())
}

func typesigi(m *F, e interface{}, embedded bool) {
	str := writer(m, !embedded)
	switch t := e.(type) {
	case *ast.ArrayType:
		str("[")
		if s, ok := t.Len.(*ast.Ident); ok {
			str(s.Name)
		}
		str("]")
		typesigi(m, t.Elt, embedded)
	case *ast.ChanType:
		if t.Dir == ast.RECV {
			str("<-")
		}
		str("chan")
		if t.Dir == ast.SEND {
			str("<-")
		}
		str(" ")
		typesigi(m, t.Value, embedded)
	case *ast.Ellipsis:
		str("...")
		typesigi(m, t.Elt, embedded)
	case *ast.MapType:
		str("map[")
		typesigi(m, t.Key, embedded)
		str("]")
		typesigi(m, t.Value, embedded)
	case *ast.StarExpr:
		str("*")
		typesigi(m, t.X, embedded)
	case *ast.Ident: //named type
		str(t.Name)
	case *ast.FuncType:
		str("func")
		Func(m, t, false)
	case *ast.InterfaceType:
		str("interface{")
		methods(m, t.Methods, true)
		str("}")
	case *ast.StructType:
		m.WriteString("struct{")
		fields(m, t.Fields, ";")
		str("}")
	case *ast.SelectorExpr:
		typesigi(m, t.X, embedded)
		str(".")
		typesigi(m, t.Sel, embedded)
	}
}
