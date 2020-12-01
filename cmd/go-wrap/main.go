package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"strings"
	"unicode"

	"github.com/romanyx/wrap"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"
)

const (
	usg = `go-wrap -p <package> -t <template> <type>
Examples:
go-wrap -p io -t ./metrics Reader
`
)

var (
	templatePath = flag.String("t", "", "template path")
	pkgPath      = flag.String("p", "", "package path")
	destPath     = flag.String("o", "", "destination path")
)

var funcMap = map[string]interface{}{
	"camelize":   camelize,
	"capitalize": strings.Title,
	"downcase":   strings.ToLower,
	"uppercase":  strings.ToUpper,
}

func main() {
	flag.Parse()
	conf := packages.Config{
		Mode:  packages.LoadSyntax,
		Tests: false,
	}

	checkFlags()

	dir, err := os.Getwd()
	if err != nil {
		exit(fmt.Errorf("get pwd: %w", err))
	}

	pkg, err := pwdPackage(&conf, dir)
	if err != nil {
		exit(fmt.Errorf("load pwd package: %w", err))
	}

	pkgs, err := packages.Load(&conf, *pkgPath)
	if err != nil {
		exit(fmt.Errorf("load package: %w", err))
	}

	t := wrap.Parse(pkg, pkgs, flag.Arg(0))

	data, err := ioutil.ReadFile(*templatePath)
	if err != nil {
		exit(fmt.Errorf("read template path: %w", err))
	}
	tmpl := template.Must(
		template.New("base").
			Funcs(funcMap).
			Parse(completeTemplate(pkg, data)))

	var buf bytes.Buffer
	tmpl.Execute(&buf, &t)

	pretty, err := imports.Process("", buf.Bytes(), &imports.Options{
		AllErrors: true, Comments: true, TabIndent: true, TabWidth: 8,
	})
	if err != nil {
		fmt.Println(buf.String())
		exit(fmt.Errorf("format source: %w", err))
	}

	err = ioutil.WriteFile(*destPath, pretty, 0644)
	if err != nil {
		exit(fmt.Errorf("write file: %w", err))
	}
}

func completeTemplate(pkg string, data []byte) string {
	return fmt.Sprintf(
		"package %s\n\n%s\n\n%s",
		pkg,
		"//go:generate go-wrap "+strings.Join(os.Args[1:], " "),
		data,
	)
}

func pwdPackage(cfg *packages.Config, path string) (string, error) {
	pkgs, err := packages.Load(cfg, path)
	if err != nil {
		return "", err
	}

	for _, pkg := range pkgs {
		return pkg.Name, nil
	}

	return "", errors.New("no packages")
}

func checkFlags() {
	if len(flag.Args()) != 1 {
		usage()
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}

func usage() {
	fmt.Fprint(os.Stderr, usg)
	os.Exit(2)
}

func camelize(s string) string {
	for i, v := range s {
		return string(unicode.ToLower(v)) + s[i+1:]
	}
	return ""
}
