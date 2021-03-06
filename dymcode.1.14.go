// +build go1.14
// +build !go1.16

package goloader

const (
	R_PCREL = 16
	// R_TLS_LE, used on 386, amd64, and ARM, resolves to the offset of the
	// thread-local symbol from the thread local base and is used to implement the
	// "local exec" model for tls access (r.Sym is not set on intel platforms but is
	// set to a TLS symbol -- runtime.tlsg -- in the linker when externally linking).
	R_TLS_LE = 17
	// R_METHODOFF resolves to a 32-bit offset from the beginning of the section
	// holding the data being relocated to the referenced symbol.
	// It is a variant of R_ADDROFF used when linking from the uncommonType of a
	// *rtype, and may be set to zero by the linker if it determines the method
	// text is unreachable by the linked program.
	R_METHODOFF = 25

	//not used, only adapter golang 1.16
	R_USEIFACE       = 0x10000000 - 3
	R_USEIFACEMETHOD = 0x10000000 - 2
	R_ADDRCUOFF      = 0x10000000 - 1
)

// copy from $GOROOT/src/cmd/internal/objabi/symkind.go
const (
	// An otherwise invalid zero value for the type
	Sxxx = iota
	// Executable instructions
	STEXT
	// Read only static data
	SRODATA
	// Static data that does not contain any pointers
	SNOPTRDATA
	// Static data
	SDATA
	// Statically data that is initially all 0s
	SBSS
	// Statically data that is initially all 0s and does not contain pointers
	SNOPTRBSS
	// Thread-local data that is initially all 0s
	STLSBSS
	// Debugging data
	SDWARFINFO
	SDWARFRANGE
	SDWARFLOC
	SDWARFLINES
	// ABI alias. An ABI alias symbol is an empty symbol with a
	// single relocation with 0 size that references the native
	// function implementation symbol.
	//
	// TODO(austin): Remove this and all uses once the compiler
	// generates real ABI wrappers rather than symbol aliases.
	SABIALIAS
	// Coverage instrumentation counter for libfuzzer.
	SLIBFUZZER_EXTRA_COUNTER
	// Update cmd/link/internal/sym/AbiSymKindToSymKind for new SymKind values.

)

func addStackObject(codereloc *CodeReloc, funcname string, symbolMap map[string]uintptr) (err error) {
	return _addStackObject(codereloc, funcname, symbolMap)
}

func addDeferReturn(codereloc *CodeReloc, _func *_func) (err error) {
	return _addDeferReturn(codereloc, _func)
}

func _buildModule(codereloc *CodeReloc, codeModule *CodeModule) {
	codeModule.module.filetab = codereloc.filetab
}
