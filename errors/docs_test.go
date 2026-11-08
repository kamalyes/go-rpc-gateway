/*
 * @Author: kamalyes 501893067@qq.com
 * @Date: 2026-11-08 00:00:00
 * @LastEditors: kamalyes 501893067@qq.com
 * @LastEditTime: 2026-11-08 00:00:00
 * @FilePath: \go-rpc-gateway\errors\docs_test.go
 * @Description: 文档里引用的本包标识符必须真实存在
 *
 * Copyright (c) 2026 by kamalyes, All Rights Reserved.
 */
package errors

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// docFiles 讲本包用法的文档
var docFiles = []string{
	"README.md",
	filepath.Join("..", "docs", "ERRORS.md"),
	filepath.Join("..", "locales", "README.md"),
}

// stdlibNames 文档里出现但属于标准库 errors 的名字
//
// 文档会写「❌ 避免 errors.New」这种劝阻，指的是标准库那个函数。
// 本包同名的东西不存在，所以这里要明确放行，而不是让测试误报
//
// [EN] Names that appear as errors.X in the docs but belong to the standard library: the docs
// say things like "avoid errors.New", meaning the stdlib function. This package has no such
// identifier, so they are allowed through explicitly rather than reported as rot
var stdlibNames = map[string]bool{
	"New": true, "As": true, "Is": true, "Unwrap": true, "Join": true,
}

// TestDocsOnlyMentionRealIdentifiers 文档里的 errors.X 必须是本包真有的导出标识符
//
// 这条测试是补上的，因为文档真的腐烂过，而且是三种都齐了：
//
//   - 签名改了没跟上：文档写 ExtractRpcErrorMsg(rpcErr.Error())，而它收的是 error
//   - 标识符删了没跟上：ErrCodeAccountLoginElsewhere、ErrCodePBMessageNil 这些码移除后
//     文档还在示范
//   - 示例指向的包都不在了：有一整段示例讲 cpool/jwt/jwt.go，那个目录早就没有
//
// 前两种编译器看不见（文档不编译），靠人读也读不出来——这正是机器该管的事
//
// [EN] Every errors.X in the docs must be an identifier this package actually exports.
//
// Added because the docs really had rotted, in all three ways: a changed signature the docs
// still showed the old way, identifiers removed while the docs went on demonstrating them, and
// a whole example built on cpool/jwt, a directory that no longer exists. The compiler cannot
// see any of it, since docs do not compile
func TestDocsOnlyMentionRealIdentifiers(t *testing.T) {
	exported := exportedNames(t)
	if len(exported) == 0 {
		t.Fatal("没扫到任何导出标识符，扫描逻辑坏了 | no exported names found")
	}

	ref := regexp.MustCompile(`\berrors\.([A-Z]\w*)`)
	for _, doc := range docFiles {
		data, err := os.ReadFile(doc)
		if err != nil {
			t.Errorf("读取 %s 失败 | reading failed: %v", doc, err)
			continue
		}
		var bad []string
		for _, m := range ref.FindAllStringSubmatch(string(data), -1) {
			name := m[1]
			if exported[name] || stdlibNames[name] {
				continue
			}
			bad = append(bad, name)
		}
		if len(bad) > 0 {
			sort.Strings(bad)
			t.Errorf("%s 引用了本包没有的标识符：%s | the doc references identifiers this package does not export",
				doc, strings.Join(dedupe(bad), "、"))
		}
	}
}

// exportedNames 本包所有顶层导出标识符
func exportedNames(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatalf("解析失败 | parse failed: %v", err)
	}
	pkg, ok := pkgs["errors"]
	if !ok {
		t.Fatal("没解析到 errors 包 | errors package not parsed")
	}

	out := map[string]bool{}
	for path, file := range pkg.Files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.IsExported() {
					out[d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch sp := spec.(type) {
					case *ast.TypeSpec:
						if sp.Name.IsExported() {
							out[sp.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, n := range sp.Names {
							if n.IsExported() {
								out[n.Name] = true
							}
						}
					}
				}
			}
		}
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
