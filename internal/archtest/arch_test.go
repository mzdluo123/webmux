// Package archtest enforces the controller/service layering of
// docs/architecture.md. Its rules read type-checked syntax, so import
// aliases, dot imports and function values cannot hide a violation. The
// rules run on this module (TestArchitecture) and on the deliberately
// violating fixture under testdata/fixture (rules_test.go).
package archtest

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

type layer string

const (
	controller layer = "controller"
	service    layer = "service"
	shared     layer = "shared"
	client     layer = "client"
	testOnly   layer = "test-only"
	// assemblyRoot is every package under cmd/.
	assemblyRoot layer = "assembly root"
)

// manifest classifies the packages of one module. Paths are relative to the
// module root.
type manifest struct {
	module string
	// layers holds every package under internal/.
	layers map[string]layer
	// outbound lists service packages that send HTTP requests, with the
	// reason. They may use net/http client symbols only.
	outbound map[string]string
}

type violation struct {
	file string
	line int
	rule string
	msg  string
	hint string
}

func (v violation) String() string {
	if v.line == 0 {
		return fmt.Sprintf("%s [%s] %s — %s", v.file, v.rule, v.msg, v.hint)
	}
	return fmt.Sprintf("%s:%d [%s] %s — %s", v.file, v.line, v.rule, v.msg, v.hint)
}

type program struct {
	dir  string
	m    manifest
	fset *token.FileSet
	// pkgs are the module's packages keyed by relative path.
	pkgs map[string]*packages.Package
	out  []violation
}

// load type-checks every package of the module in dir once; all rules share
// the result. Test files are not loaded: the rules govern shipped code.
func load(dir string, m manifest) (*program, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedDeps |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:   abs,
		Tests: false,
	}
	list, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}
	p := &program{dir: abs, m: m, pkgs: map[string]*packages.Package{}}
	var problems []string
	for _, pkg := range list {
		for _, e := range pkg.Errors {
			problems = append(problems, e.Error())
		}
		if rel, ok := p.rel(pkg.PkgPath); ok {
			p.pkgs[rel] = pkg
			p.fset = pkg.Fset
		}
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("load %s:\n%s", dir, strings.Join(problems, "\n"))
	}
	return p, nil
}

// rel returns path relative to the module, and whether it is in the module.
func (p *program) rel(path string) (string, bool) {
	if path == p.m.module {
		return "", true
	}
	rel, ok := strings.CutPrefix(path, p.m.module+"/")
	return rel, ok
}

func (p *program) layerOf(rel string) layer {
	if rel == "cmd" || strings.HasPrefix(rel, "cmd/") {
		return assemblyRoot
	}
	return p.m.layers[rel]
}

// pkgLayer is the layer of an import path; "" outside the module or for an
// unrestricted top-level package.
func (p *program) pkgLayer(path string) layer {
	if rel, ok := p.rel(path); ok {
		return p.layerOf(rel)
	}
	return ""
}

func (p *program) report(pos token.Pos, rule, hint, format string, args ...any) {
	position := p.fset.Position(pos)
	file := position.Filename
	if rel, err := filepath.Rel(p.dir, file); err == nil {
		file = filepath.ToSlash(rel)
	}
	p.out = append(p.out, violation{file, position.Line, rule, fmt.Sprintf(format, args...), hint})
}

// reportPackage records a package-level violation that has no source line.
func (p *program) reportPackage(rel, rule, hint, format string, args ...any) {
	p.out = append(p.out, violation{rel, 0, rule, fmt.Sprintf(format, args...), hint})
}

// sorted returns the violations ordered by file and line, without duplicates.
func (p *program) sorted() []violation {
	sort.Slice(p.out, func(i, j int) bool {
		a, b := p.out[i], p.out[j]
		if a.file != b.file {
			return a.file < b.file
		}
		if a.line != b.line {
			return a.line < b.line
		}
		if a.rule != b.rule {
			return a.rule < b.rule
		}
		return a.msg < b.msg
	})
	var out []violation
	for i, v := range p.out {
		if i == 0 || v != p.out[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// check runs every rule and returns the sorted violations.
func (p *program) check() []violation {
	p.checkManifest()
	for _, rel := range p.relPaths() {
		pkg := p.pkgs[rel]
		l := p.layerOf(rel)
		p.checkImports(rel, l, pkg)
		switch l {
		case controller:
			p.checkControllerUses(pkg)
		case service:
			p.checkServiceHTTP(rel, pkg)
			p.checkServiceErrors(pkg)
		}
	}
	return p.sorted()
}

func (p *program) relPaths() []string {
	paths := make([]string, 0, len(p.pkgs))
	for rel := range p.pkgs {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	return paths
}

// R0: every package under internal/ is classified, and no entry is stale.
func (p *program) checkManifest() {
	for _, rel := range p.relPaths() {
		if strings.HasPrefix(rel, "internal/") && p.m.layers[rel] == "" {
			p.reportPackage(rel, "R0", "add it to the layer manifest in internal/archtest/layers_test.go", "package %s is not classified", rel)
		}
	}
	for rel := range p.m.layers {
		if _, ok := p.pkgs[rel]; !ok {
			p.reportPackage(rel, "R0", "remove the stale entry from the layer manifest", "manifest lists %s, which does not exist", rel)
		}
	}
	for rel := range p.m.outbound {
		if p.m.layers[rel] != service {
			p.reportPackage(rel, "R0", "only service packages register outbound HTTP", "outbound registration %s is not a service package", rel)
		}
	}
}

func isWebSocket(path string) bool {
	return path == "github.com/coder/websocket" || strings.HasPrefix(path, "github.com/coder/websocket/")
}

// controllerForbiddenImport names the process, syscall, PTY and persistence
// packages a controller must reach through a service.
func (p *program) controllerForbiddenImport(path string) bool {
	switch {
	case path == "os/exec", path == "syscall":
		return true
	case path == "golang.org/x/sys" || strings.HasPrefix(path, "golang.org/x/sys/"):
		return true
	case path == "github.com/creack/pty" || strings.HasPrefix(path, "github.com/creack/pty/"):
		return true
	case path == p.m.module+"/internal/statefile":
		return true
	}
	return false
}

// R1 dependency direction, R2 the api package's importers and R3 the
// controller's forbidden imports.
func (p *program) checkImports(rel string, l layer, pkg *packages.Package) {
	api := p.m.module + "/internal/api"
	for _, file := range pkg.Syntax {
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			target := p.pkgLayer(path)
			if path == api && l != assemblyRoot && rel != "internal/api" {
				p.report(spec.Pos(), "R2", "only cmd/ wires the HTTP handler; depend on internal/contract or a shared package instead", "%s imports internal/api", rel)
				continue
			}
			switch l {
			case service:
				switch {
				case target == controller || target == assemblyRoot:
					p.report(spec.Pos(), "R1", "services must not depend on transport or wiring; move the shared part to a shared package", "service %s imports %s %s", rel, target, path)
				case isWebSocket(path), path == "net/http/httptest":
					p.report(spec.Pos(), "R1", "WebSocket and HTTP test servers belong to the controller", "service %s imports %s", rel, path)
				}
			case shared, client:
				if target == controller || target == service {
					p.report(spec.Pos(), "R1", "move the needed code into a shared package (for example internal/pathutil or internal/contract)", "%s package %s imports %s %s", l, rel, target, path)
				}
			case controller:
				if p.controllerForbiddenImport(path) {
					p.report(spec.Pos(), "R3", "move process, PTY and persistence work into a service and call its method", "controller %s imports %s", rel, path)
				}
			}
		}
	}
}

// osFilesystem are the os functions that read, create, change or remove
// filesystem entries.
var osFilesystem = map[string]bool{
	"ReadDir": true, "Stat": true, "Lstat": true, "Open": true, "OpenFile": true, "OpenInRoot": true,
	"OpenRoot": true, "Create": true, "CreateTemp": true, "Mkdir": true, "MkdirAll": true, "MkdirTemp": true,
	"Remove": true, "RemoveAll": true, "Rename": true, "Chmod": true, "Chown": true, "Lchown": true,
	"Chtimes": true, "ReadFile": true, "WriteFile": true, "Symlink": true, "Link": true, "Readlink": true,
	"Truncate": true, "CopyFS": true, "DirFS": true,
}

// R4 filesystem access and R5 service package-level functions in a
// controller. Every use of the function object counts, so calls through an
// alias, a dot import or a function value are all reported.
func (p *program) checkControllerUses(pkg *packages.Package) {
	for id, obj := range pkg.TypesInfo.Uses {
		fn, ok := obj.(*types.Func)
		if !ok || fn.Pkg() == nil {
			continue
		}
		fn = fn.Origin()
		sig, _ := fn.Type().(*types.Signature)
		if sig == nil || sig.Recv() != nil {
			continue
		}
		path := fn.Pkg().Path()
		switch {
		case path == "os" && osFilesystem[fn.Name()]:
			p.report(id.Pos(), "R4", "move the filesystem work into a service method (for example internal/files)", "controller uses os.%s", fn.Name())
		case p.pkgLayer(path) == service:
			p.report(id.Pos(), "R5", "call a method of the injected service; construct services in cmd/", "controller uses service function %s.%s", fn.Pkg().Name(), fn.Name())
		}
	}
}

// httpServer are net/http symbols that only an HTTP server uses.
var httpServer = map[string]bool{
	"ResponseWriter": true, "Handler": true, "HandlerFunc": true, "ServeMux": true, "NewServeMux": true,
	"Server": true, "Error": true, "Redirect": true, "MaxBytesReader": true, "MaxBytesHandler": true,
	"NotFound": true, "NotFoundHandler": true, "ListenAndServe": true, "ListenAndServeTLS": true,
	"Serve": true, "ServeTLS": true, "ServeContent": true, "ServeFile": true, "FileServer": true,
	"Handle": true, "HandleFunc": true, "ResponseController": true, "NewResponseController": true,
	"Flusher": true, "Hijacker": true, "StripPrefix": true, "TimeoutHandler": true, "SetCookie": true,
}

// httpClient are the net/http symbols a registered outbound service may use.
func httpClient(name string) bool {
	switch name {
	case "Client", "NewRequest", "NewRequestWithContext", "Request", "Response", "Header",
		"RoundTripper", "Transport", "DefaultTransport", "DefaultClient", "NoBody", "StatusText",
		"ErrUseLastResponse", "ProxyFromEnvironment":
		return true
	}
	return strings.HasPrefix(name, "Status") || strings.HasPrefix(name, "Method")
}

func isHTTPRequestPointer(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "net/http" && named.Obj().Name() == "Request"
}

// R6: services are transport-agnostic. Server symbols and *http.Request
// parameters are always reported; other net/http symbols only outside the
// registered outbound packages, and there only beyond the client symbols.
func (p *program) checkServiceHTTP(rel string, pkg *packages.Package) {
	_, outbound := p.m.outbound[rel]
	for id, obj := range pkg.TypesInfo.Uses {
		if obj.Pkg() == nil || obj.Pkg().Path() != "net/http" || obj.Parent() != obj.Pkg().Scope() {
			continue
		}
		name := obj.Name()
		switch {
		case httpServer[name]:
			p.report(id.Pos(), "R6", "keep HTTP serving in the controller; return data and error kinds instead", "service uses server symbol http.%s", name)
		case !outbound:
			p.report(id.Pos(), "R6", "register the package as outbound in layers_test.go if it sends HTTP requests", "unregistered service uses http.%s", name)
		case !httpClient(name):
			p.report(id.Pos(), "R6", "outbound services may use net/http client symbols only", "outbound service uses http.%s", name)
		}
	}
	// Declared functions only: an outbound client hook such as
	// http.Client.CheckRedirect is a literal that must take *http.Request.
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			for _, field := range fn.Type.Params.List {
				if isHTTPRequestPointer(pkg.TypesInfo.TypeOf(field.Type)) {
					p.report(field.Pos(), "R6", "parse the request in the controller and pass plain values", "service function %s takes *http.Request", fn.Name.Name)
				}
			}
		}
	}
}

var errorInterface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

func isInteger(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Info()&types.IsInteger != 0
}

// R7: services report error kinds, never HTTP statuses: no integer field in
// an error type and no Status function or method returning an integer.
func (p *program) checkServiceErrors(pkg *packages.Package) {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					obj := pkg.TypesInfo.Defs[ts.Name]
					if obj == nil {
						continue
					}
					t := obj.Type()
					if !types.Implements(t, errorInterface) && !types.Implements(types.NewPointer(t), errorInterface) {
						continue
					}
					st, ok := t.Underlying().(*types.Struct)
					if !ok {
						continue
					}
					for i := range st.NumFields() {
						if field := st.Field(i); isInteger(field.Type()) {
							p.report(field.Pos(), "R7", "wrap a sentinel error kind and map it to a status in internal/api/errors.go", "error type %s has integer field %s", ts.Name.Name, field.Name())
						}
					}
				}
			case *ast.FuncDecl:
				if decl.Name.Name != "Status" {
					continue
				}
				sig, _ := pkg.TypesInfo.Defs[decl.Name].Type().(*types.Signature)
				if sig != nil && sig.Results().Len() == 1 && isInteger(sig.Results().At(0).Type()) {
					p.report(decl.Name.Pos(), "R7", "return an error kind; the controller owns HTTP statuses", "service declares Status returning an integer")
				}
			}
		}
	}
}
