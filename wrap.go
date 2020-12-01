package wrap

import (
	"go/types"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

// Param is a method parameters.
type Param struct {
	Name     string
	Type     string
	Variadic bool
}

// Pass returns a name of the parameter
// If parameter is variadic it returns a name followed by a ...
func (p Param) Pass() string {
	if p.Variadic {
		return p.Name + "..."
	}
	return p.Name
}

// Method representation.
type Method struct {
	Name    string
	Params  []Param
	Results []Param

	ReturnsError   bool
	AcceptsContext bool
}

// Signature returns comma separated method's params followed by the comma separated
// method's results
func (m Method) Signature() string {
	params := []string{}
	for _, p := range m.Params {
		pass := p.Name + " " + p.Type
		if p.Variadic {
			pass = strings.Replace(pass, "[]", "...", 1)
		}
		params = append(params, pass)
	}

	results := []string{}
	for _, r := range m.Results {
		results = append(results, r.Type)
	}

	return "(" + strings.Join(params, ", ") + ") (" + strings.Join(results, ", ") + ")"
}

// Declaration returns a method name followed by it's signature
func (m Method) Declaration() string {
	return m.Name + m.Signature()
}

// Call returns a string with the method call
func (m Method) Call() string {
	params := []string{}
	for _, p := range m.Params {
		params = append(params, p.Pass())
	}

	return m.Name + "(" + strings.Join(params, ", ") + ")"
}

// ParamsNames returns a list of method params names
func (m Method) ParamsNames() string {
	ss := []string{}
	for _, p := range m.Params {
		ss = append(ss, p.Name)
	}
	return strings.Join(ss, ", ")
}

// ResultsNames returns a list of method results names
func (m Method) ResultsNames() string {
	ss := []string{}
	for _, r := range m.Results {
		ss = append(ss, r.Name)
	}
	return strings.Join(ss, ", ")
}

// ParamsStruct returns a struct type with fields corresponding
// to the method params
func (m Method) ParamsStruct() string {
	ss := []string{}
	for _, p := range m.Params {
		if p.Variadic {
			ss = append(ss, p.Name+" "+strings.Replace(p.Type, "...", "[]", 1))
		} else {
			ss = append(ss, p.Name+" "+p.Type)
		}
	}
	return "struct{\n" + strings.Join(ss, "\n ") + "}"
}

// ResultsStruct returns a struct type with fields corresponding
// to the method results
func (m Method) ResultsStruct() string {
	ss := []string{}
	for _, r := range m.Results {
		ss = append(ss, r.Name+" "+r.Type)
	}
	return "struct{\n" + strings.Join(ss, "\n ") + "}"
}

// ParamsMap returns a string representation of the map[string]interface{}
// filled with method's params
func (m Method) ParamsMap() string {
	ss := []string{}
	for _, p := range m.Params {
		ss = append(ss, `"`+p.Name+`": `+p.Name)
	}
	return "map[string]interface{}{\n" + strings.Join(ss, ",\n ") + "}"
}

// ResultsMap returns a string representation of the map[string]interface{}
// filled with method's results
func (m Method) ResultsMap() string {
	ss := []string{}
	for _, r := range m.Results {
		ss = append(ss, `"`+r.Name+`": `+r.Name)
	}
	return "map[string]interface{}{\n" + strings.Join(ss, ",\n ") + "}"
}

// HasParams returns true if method has params
func (m Method) HasParams() bool {
	return len(m.Params) > 0
}

// HasResults returns true if method has results
func (m Method) HasResults() bool {
	return len(m.Results) > 0
}

// ReturnStruct returns return statement with the return params
// taken from the structName
func (m Method) ReturnStruct(structName string) string {
	if len(m.Results) == 0 {
		return "return"
	}

	ss := []string{}
	for _, r := range m.Results {
		ss = append(ss, structName+"."+r.Name)
	}
	return "return " + strings.Join(ss, ", ")
}

// Type is a type representation to be decorated.
type Type struct {
	Type    string
	PwdPkg  string
	Pkg     string
	Methods []Method

	IsInterface bool
}

// Camelize with first letter as downcase.
func (t Type) Camelize() string {
	for i, v := range t.Type {
		return string(unicode.ToLower(v)) + t.Type[i+1:]
	}
	return ""
}

// Base returns base for a type. For concrete
// type it would be an pointer to a type, and for
// interface it would be value.
func (t Type) Base() string {
	base := t.Type
	if t.Pkg != t.PwdPkg {
		base = t.Pkg + "." + t.Type
	}

	if t.IsInterface {
		return base
	}

	return "*" + base
}

// Parse finds type which going to be wraped and returns
// contructed type.
func Parse(pwdPkg string, pkgs []*packages.Package, name string) Type {
	t := Type{
		Type:   name,
		PwdPkg: pwdPkg,
	}

	methodCheck := make(map[string]bool)
	for _, pkg := range pkgs {
		t.Pkg = pkg.Name
		l := pkg.Types.Scope().Lookup(name)
		if l == nil {
			continue
		}
		tt := l.Type()
		t.IsInterface = types.IsInterface(tt)

		for _, tp := range []types.Type{
			tt,
			types.NewPointer(tt),
		} {
			mset := types.NewMethodSet(tp)
			for i := 0; i < mset.Len(); i++ {
				var m Method
				s := mset.At(i)
				m.Name = s.Obj().Name()
				if isPrivate(m.Name) || methodCheck[m.Name] {
					continue
				}

				methodCheck[m.Name] = true

				sign := s.Type().(*types.Signature)
				params := sign.Params()
				paramsUsedNames := make(map[string]bool)
				for i := 0; i < params.Len(); i++ {
					v := params.At(i)
					param := newParam(pwdPkg, v, paramsUsedNames)
					if i == params.Len()-1 && sign.Variadic() {
						param.Variadic = true
					}
					if i == 0 && param.Type == "context.Context" {
						m.AcceptsContext = true
					}

					m.Params = append(m.Params, param)
				}

				results := sign.Results()
				resultsUsedNames := make(map[string]bool)
				for i := 0; i < results.Len(); i++ {
					v := results.At(i)
					param := newParam(pwdPkg, v, resultsUsedNames)
					m.Results = append(m.Results, param)

					if i == results.Len()-1 && isErrorType(v.Type()) {
						m.ReturnsError = true
					}
				}

				t.Methods = append(t.Methods, m)
			}
		}
	}

	return t
}

func newParam(
	pwdPkg string,
	v *types.Var,
	usedNames map[string]bool,
) Param {
	tName := typeName(pwdPkg, v.Type())
	name := v.Name()
	if name == "" {
		name = genName(tName, 0, usedNames)
	}
	usedNames[name] = true

	p := Param{
		Name: name,
		Type: tName,
	}
	return p
}

func genName(t string, n int, usedNames map[string]bool) string {
	prefix := variableForType(t)
	if n > 0 {
		prefix = prefix + strconv.Itoa(n)
	}

	if usedNames[prefix] {
		return genName(prefix, n+1, usedNames)
	}

	return prefix
}

func variableForType(t string) string {
	if t == "context.Context" {
		return "ctx"
	}

	if t == "error" {
		return "err"
	}

	parts := strings.Split(t, ".")
	if len(parts) > 1 {
		for _, v := range strings.ToLower(parts[len(parts)-1]) {
			return string(v)
		}
	}

	for _, v := range strings.ToLower(t) {
		return string(v)
	}

	return "v"
}

func isErrorType(t types.Type) bool {
	return t.String() == "error" &&
		t.Underlying().String() == "interface{Error() string}"
}

func typeName(pkg string, t types.Type) string {
	return types.TypeString(
		t,
		func(other *types.Package) string {
			if pkg == other.Name() {
				return ""
			}

			return other.Name()
		})
}

func isPrivate(s string) bool {
	return unicode.IsLower(rune(s[0]))
}
