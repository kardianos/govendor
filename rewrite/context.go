package rewrite

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Context struct {
	GopathList []string
	Goroot     string

	RootDir        string
	RootGopath     string
	RootImportPath string

	VendorFile *VendorFile

	parserFileSet  *token.FileSet
	packageCache   map[string]*Package
	packageUnknown map[string]struct{}
}

func NewContextWD() (*Context, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := findRoot(wd)
	if err != nil {
		return nil, err
	}

	vf, err := readVendorFile(root)
	if err != nil {
		return nil, err
	}

	// Get GOROOT. First check ENV, then run "go env" and find the GOROOT line.
	cmd := exec.Command("go", "env")
	goEnv, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	var goroot = ""
	const gorootLookFor = `GOROOT="`
	for _, line := range strings.Split(string(goEnv), "\n") {
		if strings.HasPrefix(line, gorootLookFor) == false {
			continue
		}
		goroot = strings.TrimPrefix(line, gorootLookFor)
		goroot = strings.TrimSuffix(goroot, `"`)
		goroot = filepath.Join(goroot, "src")
		break
	}
	if goroot == "" {
		return nil, ErrMissingGOROOT
	}

	// Get the GOPATHs. Prepend the GOROOT to the list.
	all := os.Getenv("GOPATH")
	if len(all) == 0 {
		return nil, ErrMissingGOPATH
	}
	gopathList := filepath.SplitList(all)
	gopathGoroot := make([]string, 0, len(gopathList)+1)
	gopathGoroot = append(gopathGoroot, goroot)
	for _, gopath := range gopathList {
		gopathGoroot = append(gopathGoroot, filepath.Join(gopath, "src")+string(filepath.Separator))
	}

	ctx := &Context{
		RootDir:    root,
		GopathList: gopathGoroot,
		Goroot:     goroot,

		VendorFile: vf,

		parserFileSet:  token.NewFileSet(),
		packageCache:   make(map[string]*Package),
		packageUnknown: make(map[string]struct{}),
	}
	ctx.RootImportPath, ctx.RootGopath, err = ctx.findImportPath(root)
	if err != nil {
		return nil, err
	}
	return ctx, nil
}

// findImportDir finds the absolute directory. If gopath is not empty, it is used.
func (ctx *Context) findImportDir(importPath, useGopath string) (dir, gopath string, err error) {
	paths := ctx.GopathList
	if len(useGopath) != 0 {
		paths = []string{useGopath}
	}
	if importPath == "builtin" || importPath == "unsafe" {
		return filepath.Join(ctx.Goroot, importPath), ctx.Goroot, nil
	}
	for _, gopath = range paths {
		dir := filepath.Join(gopath, importPath)
		fi, err := os.Stat(dir)
		if os.IsNotExist(err) {
			continue
		}
		if fi.IsDir() == false {
			continue
		}
		return dir, gopath, nil
	}
	return "", "", ErrNotInGOPATH{importPath}
}

// findImportPath takes a absolute directory and returns the import path and go path.
func (ctx *Context) findImportPath(dir string) (importPath, gopath string, err error) {
	for _, gopath := range ctx.GopathList {
		if strings.HasPrefix(dir, gopath) {
			importPath = strings.TrimPrefix(dir, gopath)
			if filepath.Separator == '\\' {
				importPath = strings.Replace(importPath, `\`, "/", -1)
			}
			return importPath, gopath, nil
		}
	}
	return "", "", ErrNotInGOPATH{dir}
}

type Package struct {
	Dir        string
	ImportPath string
	Files      []*File
	Status     ListStatus
}
type File struct {
	Path    string
	Imports []string
}

func (ctx *Context) LoadImports() error {
	err := filepath.Walk(ctx.RootDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() && info.Name()[0] == '.' {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		return ctx.addFileImports(path, ctx.RootGopath)
	})
	if err != nil {
		return err
	}
	for _, pkg := range ctx.packageCache {
		for _, f := range pkg.Files {
			for _, imp := range f.Imports {
				if _, found := ctx.packageCache[imp]; !found {
					ctx.packageUnknown[imp] = struct{}{}
				}
			}
		}
	}
	err = ctx.resolveUnknown()
	if err != nil {
		return err
	}

	for _, pkg := range ctx.packageCache {
		fmt.Printf("PKG: %s\n", pkg.ImportPath)
		for _, f := range pkg.Files {
			fmt.Printf("\tFile: %s\n", f.Path)
			for _, imp := range f.Imports {
				fmt.Printf("\t\tImport: %v\n", imp)
			}
		}
	}
	for importPath := range ctx.packageUnknown {
		fmt.Printf("Waiting: %s\n", importPath)
	}

	return nil
}

func (ctx *Context) addFileImports(path, gopath string) error {
	if strings.HasSuffix(path, ".go") == false {
		return nil
	}
	f, err := parser.ParseFile(ctx.parserFileSet, path, nil, parser.ImportsOnly)
	if err != nil {
		return err
	}

	dir, _ := filepath.Split(path)
	importPath := strings.TrimPrefix(dir, gopath)
	if filepath.Separator == '\\' {
		importPath = strings.Replace(importPath, `\`, "/", -1)
	}
	importPath = strings.TrimPrefix(importPath, "/")
	importPath = strings.TrimSuffix(importPath, "/")

	pkg, found := ctx.packageCache[importPath]
	if !found {
		pkg = &Package{
			Dir:        dir,
			ImportPath: importPath,
		}
		ctx.packageCache[importPath] = pkg

		if _, found := ctx.packageUnknown[importPath]; found {
			delete(ctx.packageUnknown, importPath)
		}
	}
	pf := &File{
		Path:    path,
		Imports: make([]string, len(f.Imports)),
	}
	pkg.Files = append(pkg.Files, pf)
	for i := range f.Imports {
		imp := f.Imports[i].Path.Value
		imp = strings.TrimSuffix(strings.TrimPrefix(imp, `"`), `"`)
		pf.Imports[i] = imp

		if _, found := ctx.packageCache[imp]; !found {
			ctx.packageUnknown[imp] = struct{}{}
		}
	}

	return nil
}

func (ctx *Context) resolveUnknown() error {
top:
	for len(ctx.packageUnknown) > 0 {
		for importPath := range ctx.packageUnknown {
			dir, gopath, err := ctx.findImportDir(importPath, "")
			if err != nil {
				return err
			}
			if gopath == ctx.Goroot {
				ctx.packageCache[importPath] = &Package{
					Dir:        dir,
					ImportPath: importPath,
					Status:     StatusStd,
				}
				delete(ctx.packageUnknown, importPath)
				continue top
			}
			df, err := os.Open(dir)
			if err != nil {
				return err
			}
			info, err := df.Readdir(-1)
			if err != nil {
				return err
			}
			for _, fi := range info {
				if fi.IsDir() {
					continue
				}
				if fi.Name()[0] == '.' {
					continue
				}
				path := filepath.Join(dir, fi.Name())
				err = ctx.addFileImports(path, gopath)
				if err != nil {
					return err
				}
			}
			continue top
		}
	}
	return nil
}
