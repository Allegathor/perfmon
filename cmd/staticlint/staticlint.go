package main

import (
	"github.com/Allegathor/perfmon/cmd/staticlint/osexitcheck"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/quickfix"
	"honnef.co/go/tools/staticcheck"
)

// to lint files in a current directory run `staticlint`
// to lint files in a current directory and its subdirectories run `staticlint ./...`
// to get help and description of each analyzer run `staticlint help`
func main() {
	analyzers := []*analysis.Analyzer{
		asmdecl.Analyzer,      // report mismatches between assembly files and Go declarations
		assign.Analyzer,       // check for useless assignments
		atomic.Analyzer,       // check for common mistakes using the sync/atomic package
		bools.Analyzer,        // check for common mistakes involving boolean operators
		buildtag.Analyzer,     // check that +build tags are well-formed and correctly located
		cgocall.Analyzer,      // detect some violations of the cgo pointer passing rules
		composite.Analyzer,    // check for unkeyed composite literals
		copylock.Analyzer,     // check for locks erroneously passed by value
		httpresponse.Analyzer, // check for mistakes using HTTP responses
		loopclosure.Analyzer,  // check references to loop variables from within nested functions
		lostcancel.Analyzer,   // check cancel func returned by context.WithCancel is called
		nilfunc.Analyzer,      // check for useless comparisons between functions and nil
		printf.Analyzer,       // check consistency of Printf format strings and arguments
		shift.Analyzer,        // check for shifts that equal or exceed the width of the integer
		stdmethods.Analyzer,   // check signature of methods of well-known interfaces
		structtag.Analyzer,    // check that struct field tags conform to reflect.StructTag.Get
		tests.Analyzer,        // check for common mistaken usages of tests and examples
		unmarshal.Analyzer,    // report passing non-pointer or non-interface values to unmarshal
		unreachable.Analyzer,  // check for unreachable code
		unsafeptr.Analyzer,    // check for invalid conversions of uintptr to unsafe.Pointer
		unusedresult.Analyzer, // check for unused results of calls to some functions
		osexitcheck.Analyzer,  // check for os.Exit() call, which isn't allowed
	}
	for _, ls := range [][]*lint.Analyzer{
		staticcheck.Analyzers, // the SA category of checks, codenamed staticcheck, contains all checks that are concerned with the correctness of code.
		quickfix.Analyzers,    // the QF category of checks, codenamed quickfix, contains checks that are used as part of gopls for automatic refactorings
	} {
		for _, v := range ls {
			analyzers = append(analyzers, v.Analyzer)
		}
	}

	multichecker.Main(
		analyzers...,
	)
}
