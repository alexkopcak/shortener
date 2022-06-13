package main

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/atomicalign"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/fieldalignment"
	"golang.org/x/tools/go/analysis/passes/findcall"
	"golang.org/x/tools/go/analysis/passes/framepointer"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/pkgfact"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/reflectvaluecompare"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sigchanyzer"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"golang.org/x/tools/go/analysis/passes/unusedwrite"
	"golang.org/x/tools/go/analysis/passes/usesgenerics"
	"honnef.co/go/tools/staticcheck"
)

var OsExitCheckAnalyzer = &analysis.Analyzer{
	Name: "osexitcheck",
	Doc:  "check for call os.Exit() in main func",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	var file *ast.File
	for _, value := range pass.Files {
		if value.Name.Name == "main.go" {
			ast.Inspect(file, func(n ast.Node) bool {
				if expr, ok := n.(*ast.CallExpr); ok {
					if fun, ok := expr.Fun.(*ast.SelectorExpr); ok {
						if ident, ok := fun.X.(*ast.Ident); ok {
							if (ident.Name == "os") && (fun.Sel.Name == "Exit") {
								pass.Reportf(fun.Pos(), "expression returns os.Exit function position")
							}
						}
					}
				}
				return true
			})
		}
	}
	return nil, nil
}

func main() {
	shortnerChecks := []*analysis.Analyzer{
		asmdecl.Analyzer,             // reports mismatches between assembly files and Go declarations
		assign.Analyzer,              // detects useless assignments
		atomic.Analyzer,              // checks for common mistakes using the sync/atomic package
		atomicalign.Analyzer,         // checks for non-64-bit-aligned arguments to sync/atomic functions
		bools.Analyzer,               // detects common mistakes involving boolean operators
		buildssa.Analyzer,            // constructs the SSA representation of an error-free package and returns the set of all functions within it.
		buildtag.Analyzer,            // checks build tags
		cgocall.Analyzer,             // detects some violations of the cgo pointer passing rules
		composite.Analyzer,           // checks for unkeyed composite literals
		copylock.Analyzer,            // checks for locks erroneously passed by value
		ctrlflow.Analyzer,            // provides a syntactic control-flow graph (CFG) for the body of a function
		deepequalerrors.Analyzer,     // checks for the use of reflect.DeepEqual with error values
		errorsas.Analyzer,            // checks that the second argument to errors.As is a pointer to a type implementing error
		fieldalignment.Analyzer,      // detects structs that would use less memory if their fields were sorted
		findcall.Analyzer,            // serves as a trivial example and test of the Analysis API
		framepointer.Analyzer,        // reports assembly code that clobbers the frame pointer before saving it
		httpresponse.Analyzer,        // checks for mistakes using HTTP responses
		ifaceassert.Analyzer,         // flags impossible interface-interface type assertions
		inspect.Analyzer,             // provides an AST inspector (golang.org/x/tools/go/ast/inspector.Inspector) for the syntax trees of a package
		loopclosure.Analyzer,         // checks for references to enclosing loop variables from within nested functions
		lostcancel.Analyzer,          // checks for failure to call a context cancellation function
		nilfunc.Analyzer,             // checks for useless comparisons against nil
		nilness.Analyzer,             // inspects the control-flow graph of an SSA function and reports errors such as nil pointer dereferences and degenerate nil pointer comparisons.
		pkgfact.Analyzer,             // demonstration and test of the package fact mechanism
		printf.Analyzer,              // checks consistency of Printf format strings and arguments
		reflectvaluecompare.Analyzer, // checks for accidentally using == or reflect.DeepEqual to compare reflect.Value values
		shadow.Analyzer,              // checks for shadowed variables
		shift.Analyzer,               // checks for shifts that exceed the width of an integer
		sigchanyzer.Analyzer,         // detects misuse of unbuffered signal as argument to signal.Notify
		sortslice.Analyzer,           // checks for calls to sort.Slice that do not use a slice type as first argument
		stdmethods.Analyzer,          // checks for misspellings in the signatures of methods similar to well-known interfaces
		stringintconv.Analyzer,       // flags type conversions from integers to strings
		structtag.Analyzer,           // checks struct field tags are well formed
		tests.Analyzer,               // checks for common mistaken usages of tests and examples
		unmarshal.Analyzer,           // checks for passing non-pointer or non-interface types to unmarshal and decode functions
		unreachable.Analyzer,         // checks for unreachable code
		unsafeptr.Analyzer,           // checks for invalid conversions of uintptr to unsafe.Pointer
		unusedresult.Analyzer,        // checks for unused results of calls to certain pure functions
		unusedwrite.Analyzer,         // checks for unused writes to the elements of a struct or array object
		usesgenerics.Analyzer,        // checks for usage of generic features
	}

	checks := map[string]bool{
		"SA":     true, // staticcheck
		"S1000":  true, // Use plain channel send or receive instead of single-case select
		"ST1005": true, // Incorrectly formatted error string
		"QT1001": true, // Apply De Morgan’s law
	}

	for _, v := range staticcheck.Analyzers {
		if checks[v.Analyzer.Name] {
			shortnerChecks = append(shortnerChecks, v.Analyzer)
		}
	}

	shortnerChecks = append(shortnerChecks, OsExitCheckAnalyzer)

	multichecker.Main(shortnerChecks...)
}
