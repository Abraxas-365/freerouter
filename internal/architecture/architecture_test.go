package architecture_test

// Architecture tests: the hexagonal rules from AGENTS.md, enforced on the
// import graph and the AST. Each new pattern gets a rule here.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const module = "github.com/Abraxas-365/freerouter"

// pkg is one non-test Go package under internal/ or cmd/.
type pkg struct {
	rel     string            // e.g. internal/apikey/apikeysvc
	imports map[string]string // import path -> first file importing it
	files   map[string]*ast.File
}

func root(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func loadPackages(t *testing.T) map[string]*pkg {
	t.Helper()
	base := root(t)
	pkgs := map[string]*pkg{}
	fset := token.NewFileSet()
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(base, top), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly|parser.ParseComments)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(base, filepath.Dir(path))
			rel = filepath.ToSlash(rel)
			p := pkgs[rel]
			if p == nil {
				p = &pkg{rel: rel, imports: map[string]string{}, files: map[string]*ast.File{}}
				pkgs[rel] = p
			}
			p.files[filepath.Base(path)] = f
			for _, imp := range f.Imports {
				ip, _ := strconv.Unquote(imp.Path.Value)
				if _, ok := p.imports[ip]; !ok {
					p.imports[ip] = filepath.Base(path)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(pkgs) < 20 {
		t.Fatalf("loaded only %d packages; is the walk rooted correctly (%s)?", len(pkgs), base)
	}
	return pkgs
}

func internalImport(ip string) (string, bool) {
	return strings.CutPrefix(ip, module+"/")
}

// isAdapter reports whether rel is internal/<domain>/adapters/<x>.
func isAdapter(rel string) bool {
	parts := strings.Split(rel, "/")
	return len(parts) == 4 && parts[0] == "internal" && parts[2] == "adapters"
}

func isModule(rel string) bool {
	parts := strings.Split(rel, "/")
	return len(parts) == 3 && parts[0] == "internal" && parts[2] == parts[1]+"module"
}

func isSvc(rel string) bool {
	parts := strings.Split(rel, "/")
	return len(parts) == 3 && parts[0] == "internal" && parts[2] == parts[1]+"svc"
}

// domains are internal/<d> packages that have an adapters/ or <d>svc child.
func domains(pkgs map[string]*pkg) map[string]bool {
	out := map[string]bool{}
	for rel := range pkgs {
		if isAdapter(rel) || isSvc(rel) || isModule(rel) {
			out["internal/"+strings.Split(rel, "/")[1]] = true
		}
	}
	return out
}

func report(t *testing.T, violations []string, rule string) {
	t.Helper()
	sort.Strings(violations)
	for _, v := range violations {
		t.Errorf("%s: %s", rule, v)
	}
}

// Concrete adapters are wired only by their module's assembler.
func TestAdaptersImportedOnlyByModules(t *testing.T) {
	var bad []string
	for rel, p := range loadPackages(t) {
		for ip, file := range p.imports {
			target, ok := internalImport(ip)
			if !ok || !isAdapter(target) {
				continue
			}
			domain := strings.Split(target, "/")[1]
			if rel == "internal/"+domain+"/"+domain+"module" {
				continue
			}
			bad = append(bad, rel+"/"+file+" imports "+target)
		}
	}
	report(t, bad, "only <domain>module may import concrete adapters")
}

// Adapters never depend on each other.
func TestAdaptersDoNotImportOtherAdapters(t *testing.T) {
	var bad []string
	for rel, p := range loadPackages(t) {
		if !isAdapter(rel) {
			continue
		}
		for ip, file := range p.imports {
			if target, ok := internalImport(ip); ok && isAdapter(target) && target != rel {
				bad = append(bad, rel+"/"+file+" imports "+target)
			}
		}
	}
	report(t, bad, "adapters must not import other adapters")
}

// Domain packages and services don't know about other domains' internals,
// modules, the composition root, or transport/driver libraries.
func TestDomainAndServiceLayers(t *testing.T) {
	pkgs := loadPackages(t)
	doms := domains(pkgs)
	forbiddenLibs := []string{"github.com/gofiber/", "github.com/jmoiron/sqlx", "github.com/lib/pq", "github.com/redis/", "net/http"}
	// Pre-existing: the permission catalog lives in internal/server, so these
	// domains import it (and Fiber transitively). Move the catalog to a
	// Fiber-free package and delete these entries; never add new ones.
	allowServer := map[string]bool{
		"internal/apikey":           true, // server.ValidPermissions/DefaultPermissions
		"internal/access":           true, // server.ValidPermissions
		"internal/apikey/apikeysvc": true, // Resolver implements server.KeyResolver
	}
	// Pre-existing by design: gateway is the proxy layer (upstream HTTP client,
	// Redis response cache live in the package); webhooksvc delivers webhooks
	// over its SSRF-hardened HTTP client.
	allowLib := map[string]bool{
		"internal/gateway net/http":                     true,
		"internal/gateway github.com/redis/go-redis/v9": true,
		"internal/webhook/webhooksvc net/http":          true,
	}
	var bad []string
	for rel, p := range pkgs {
		if !doms[rel] && !isSvc(rel) {
			continue
		}
		for ip, file := range p.imports {
			where := rel + "/" + file + " imports " + ip
			for _, lib := range forbiddenLibs {
				if (ip == strings.TrimSuffix(lib, "/") || strings.HasPrefix(ip, lib)) && !allowLib[rel+" "+ip] {
					bad = append(bad, where)
				}
			}
			target, ok := internalImport(ip)
			if !ok {
				continue
			}
			switch {
			case isAdapter(target), isModule(target), target == "internal/bootstrap":
				bad = append(bad, where)
			case target == "internal/server" && !allowServer[rel]:
				bad = append(bad, where)
			case isSvc(target) && target != rel:
				bad = append(bad, where+" (depend on the domain's ports, not another service)")
			}
		}
	}
	report(t, bad, "domain/service layer boundary")
}

// Foundation packages stay leaf-level.
func TestFoundationPackages(t *testing.T) {
	allowed := map[string][]string{
		"internal/errx":     {},
		"internal/query":    {},
		"internal/config":   {},
		"internal/identity": {"internal/errx"},
		"internal/httpx":    {"internal/query"},
		"internal/server":   {"internal/errx", "internal/config"},
	}
	pkgs := loadPackages(t)
	var bad []string
	for rel, ok := range allowed {
		p := pkgs[rel]
		if p == nil {
			t.Fatalf("package %s not found", rel)
		}
		for ip, file := range p.imports {
			target, internal := internalImport(ip)
			if !internal {
				continue
			}
			if !contains(ok, target) {
				bad = append(bad, rel+"/"+file+" imports "+target)
			}
		}
	}
	report(t, bad, "foundation packages import only their allowed internals")
}

// The IAMKit SDK is a vendor detail of the iam auth mode: only the IAMKit
// adapters, iamx, the server's IAM authenticator and the composition root
// may import it, so none/token modes never depend on it.
func TestIAMKitSDKConfined(t *testing.T) {
	allowed := map[string]string{
		"internal/iamx":                         "",
		"internal/bootstrap":                    "",
		"internal/server":                       "auth.go", // the IAM authenticator only
		"internal/access/adapters/accessiamkit": "",
		"internal/apikey/adapters/apikeyiamkit": "",
	}
	var bad []string
	for rel, p := range loadPackages(t) {
		for name, f := range p.files {
			for _, imp := range f.Imports {
				ip, _ := strconv.Unquote(imp.Path.Value)
				if !strings.HasPrefix(ip, "github.com/Abraxas-365/iamkit/") {
					continue
				}
				file, ok := allowed[rel]
				if !ok || (file != "" && file != name) {
					bad = append(bad, rel+"/"+name+" imports "+ip)
				}
			}
		}
	}
	report(t, bad, "IAMKit SDK outside its adapters")
}

// Handlers get the caller from server.Caller, never from IAMKit claims or
// raw auth headers, so they work in every FREEROUTER_AUTH mode.
func TestHandlersUseCaller(t *testing.T) {
	base := root(t)
	pkgs := loadPackages(t)
	var bad []string
	fset := token.NewFileSet()
	for rel := range pkgs {
		if !isAdapter(rel) || !strings.HasSuffix(rel, "http") {
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(base, rel, "*.go"))
		for _, path := range matches {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				v, _ := strconv.Unquote(lit.Value)
				switch strings.ToLower(v) {
				case "authorization", "x-api-key", "claims", "fiberauth.claims":
					bad = append(bad, rel+"/"+filepath.Base(path)+": "+fset.Position(lit.Pos()).String()+" reads "+strconv.Quote(v))
				}
				return true
			})
		}
	}
	report(t, bad, "HTTP adapters read the caller with server.Caller(c)")
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
