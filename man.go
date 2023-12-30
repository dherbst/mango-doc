package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/doc"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"time"
)

//BUG(jmf): Quotation marks are all wrong in postscript output.

func ovr_map(in []*section) map[string][]interface{} {
	out := map[string][]interface{}{}
	for _, s := range in {
		out[s.name] = s.paras
	}
	return out
}

type M struct {
	*F
	name, version, sec   string
	descr                []byte //short description
	sections, overd, end []*section
	overm                map[string][]interface{}
	refs                 []string
	pkg                  *ast.Package
	docs                 *doc.Package
}

func NewManPage(pkg *ast.Package, docs *doc.Package, overd []*section) *M {
	//break up the package document, extract a short description
	dvec := unstring([]byte(docs.Doc))
	var fs []byte //first sentence.
	if dvec != nil && len(dvec) > 0 {
		if p, ok := dvec[0].([][]byte); ok && len(p) > 0 {
			fs = p[0]
			//if the first paragraph is one sentence, only use it in description
			//otherwise we leave it where it is to repeat.
			if len(p) == 1 {
				dvec = dvec[1:]
			}
		}
	}
	m := &M{
		F:        Formatter(),
		name:     *name,
		version:  grep_version(pkg),
		descr:    fs,
		sections: sections(dvec),
		overd:    overd,
		overm:    ovr_map(overd),
		pkg:      pkg,
		docs:     docs,
	}
	h := -1
	for i, sec := range m.sections {
		if sec.name == "HISTORY" {
			h = i
			break
		}
	}
	if hs := get_section(m, "HISTORY", h); hs != nil {
		m.end = []*section{&section{"HISTORY", hs}}
	}
	m.WriteString(".\\\"    Automatically generated by mango-doc(1)")
	return m
}

func lit(x interface{}) []byte {
	if b, ok := x.(*ast.BasicLit); ok {
		v := b.Value
		switch b.Kind {
		case token.CHAR, token.STRING:
			v, _ = strconv.Unquote(v)
		}
		return []byte(v)
	}
	return nil
}

func grep_version(pkg *ast.Package) string {
	if *version != "" {
		return *version
	}
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			if g, ok := decl.(*ast.GenDecl); ok {
				if g.Tok == token.CONST || g.Tok == token.VAR {
					for _, s := range g.Specs {
						if v, ok := s.(*ast.ValueSpec); ok {
							for i, n := range v.Names {
								if n.Name == "Version" {
									t := v.Values[i]
									if b, ok := t.(*ast.BasicLit); ok {
										return string(lit(b))
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return ""
}

func flatten(docs *doc.Package, extras []string) <-chan string {
	out := make(chan string)
	var sub func(interface{})
	sub = func(x interface{}) {
		switch t := x.(type) {
		case []*doc.Value:
			for _, v := range t {
				out <- v.Doc
			}
		case []*doc.Func:
			for _, v := range t {
				out <- v.Doc
			}
		case []*doc.Type:
			for _, v := range t {
				out <- v.Doc
				sub(v.Consts)
				sub(v.Vars)
				sub(v.Funcs)
				sub(v.Methods)
			}
		}
	}
	go func() {
		for _, x := range extras {
			out <- x
		}
		for _, bug := range docs.Bugs {
			out <- bug
		}
		out <- docs.Doc
		sub(docs.Consts)
		sub(docs.Types)
		sub(docs.Vars)
		sub(docs.Funcs)
		close(out)
	}()
	return out
}

func (m *M) find_refs(extras []string) {
	var acc []string
	seen := map[string]bool{}
	seen[m.name+"("+m.sec+")"] = true //don't want recursive references
	for str := range flatten(m.docs, extras) {
		for _, word := range strings.Fields(str) {
			if !refrx.MatchString(word) {
				continue
			}
			switch word[strings.Index(word, "(")+1] { //check the part in the ()
			case '1', '2', '3', '4', '5', '6', '7', '8', '9', '0',
				'n', 'o', 'l', 'x', 'p':
				//okay, even though most of these are unlikely
				//and some deprecated
			default:
				//not a man page
				continue
			}
			if !seen[word] {
				seen[word] = true
				acc = append(acc, word)
			}
		}
	}
	sort.Strings(acc)
	m.refs = acc
}

func (m *M) do_header(kind string) {
	tm := time.Now().Format("2006-01-02")
	version := m.version
	if version == "" {
		version = tm
	}
	if *manual != "" {
		kind = *manual
	}
	m.WriteString(
		fmt.Sprintf("\n.TH \"%s\" %s \"%s\" \"version %s\" \"%s\"",
			m.name,
			m.sec,
			tm,
			version,
			kind,
		))
}

func (m *M) do_name() {
	m.section("NAME")
	m.WriteString(m.name)
	s := bytes.TrimSpace(m.descr)
	if len(s) > 0 {
		m.WriteString(" \\- ")
		m.Write(s) //first sentence
	}
}

func get_section(m *M, nm string, i int) (ps []interface{}) {
	ok := false
	if ps, ok = m.overm[nm]; ok {
		delete(m.overm, nm)
	} else if i != -1 {
		ps = m.sections[i].paras
	}
	//regardless of where it comes from, remove from m.sections given valid i
	switch {
	case i == -1:
		return
	case i == 0:
		m.sections = m.sections[1:]
	case i == len(m.sections)-1:
		m.sections = m.sections[:len(m.sections)-1]
	default:
		copy(m.sections[i:], m.sections[i+1:])
		m.sections = m.sections[:len(m.sections)-1]
	}
	return
}

func (m *M) do_description() {
	i := -1
	if len(m.sections) > 0 {
		i = 0
	}
	ps := get_section(m, "", i)
	if ps != nil && len(ps) > 0 {
		m.section("DESCRIPTION")
		m.paras(ps)
	}
}

func (m *M) user_sections(sx ...string) {
	for _, req := range sx {
		for i, sc := range m.sections {
			if sc.name != req {
				i = -1
			}
			if ps := get_section(m, req, i); ps != nil {
				m.section(req)
				m.paras(ps)
			}
		}
	}
}

func (m *M) remaining_user_sections() {
	for _, sec := range m.sections {
		m.section(sec.name)
		m.paras(sec.paras)
	}
	//this is horrible but beats deleting the overd sections as we go
	for _, s := range m.overd {
		if _, ok := m.overm[s.name]; ok {
			m.section(s.name)
			m.paras(s.paras)
		}
	}
}

func (m *M) do_endmatter() {
	for _, sec := range m.end {
		m.section(sec.name)
		m.paras(sec.paras)
	}
}

func (m *M) do_bugs() {
	bs := m.docs.Bugs
	if len(bs) > 0 {
		m.section("BUGS")
		m.text(bytes.TrimSpace([]byte(bs[0])))
		for _, b := range bs[1:] {
			m.PP()
			m.text(bytes.TrimSpace([]byte(b)))
		}
	}
}

func (m *M) _seealso1(s string) {
	m.WriteString(".BR ")
	piv := strings.Index(s, "(")
	m.Write(escape([]byte(s[:piv])))
	m.WriteByte(' ')
	m.WriteString(s[piv:])
}

func (m *M) do_see_also() {
	if len(m.refs) > 0 {
		m.section("SEE ALSO")
		last := len(m.refs) - 1
		for _, s := range m.refs[:last] {
			m._seealso1(s)
			m.WriteString(",\n")
		}
		m._seealso1(m.refs[last])
	}
}
