package rewrite

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type Context struct {
	Gopath         string
	GopathSrc      string
	RootImportPath string
	RootPath       string
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
	gopath, rootImportPath, err := findGOPATH(root)
	if err != nil {
		return nil, err
	}

	ctx := &Context{
		RootPath:       root,
		RootImportPath: rootImportPath,
		Gopath:         gopath,
		GopathSrc:      filepath.Join(gopath, "src"),
	}
	return ctx, nil
}

type Package struct {
	Dir        string
	ImportPath string
	Ast        *ast.Package

	Std      bool
	Internal bool
}

func (ctx *Context) LoadImports() error {
	fileset := token.NewFileSet()
	pkgLookup := map[string]*Package{}
	pkgList := []*Package{}
	err := filepath.Walk(ctx.RootPath, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() && info.Name()[0] == '.' {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") == false {
			return nil
		}
		f, err := parser.ParseFile(fileset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		name := f.Name.Name

		dir, filename := filepath.Split(path)
		importPath := strings.TrimPrefix(dir, ctx.GopathSrc)
		if filepath.Separator == '\\' {
			strings.Replace(importPath, `\`, "/", -1)
		}
		importPath = strings.TrimPrefix(importPath, "/")
		importPath = strings.TrimSuffix(importPath, "/")

		pkg, found := pkgLookup[importPath]
		if !found {
			pkg = &Package{
				Dir:        dir,
				ImportPath: importPath,
				Ast: &ast.Package{
					Name:  name,
					Files: make(map[string]*ast.File, 1),
				},
			}
			pkgLookup[importPath] = pkg
			pkgList = append(pkgList, pkg)
		}
		pkg.Ast.Files[filename] = f

		// fmt.Printf("Path: %s, Name: %s\n", relativePackagePath, name)

		return nil
	})
	if err != nil {
		return err
	}
	for _, pkg := range pkgList {
		fmt.Printf("PKG: %s >> %s\n", pkg.Ast.Name, pkg.ImportPath)
		for name, f := range pkg.Ast.Files {
			fmt.Printf("\tFile: %s\n", name)
			for _, imp := range f.Imports {
				fmt.Printf("\t\tImport: %v\n", imp.Path.Value)
			}
		}
	}

	// TODO: Make context wide package cache.
	// TODO: Determine if each package is local, internal, external, std.

	return nil
}

func (ctx *Context) LoadDir() error {
	return nil
}
