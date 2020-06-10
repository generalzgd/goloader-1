package goloader

import (
	"cmd/objfile/goobj"
	"encoding/binary"
	"errors"
	"strings"
	"unsafe"
)

//go:linkname firstmoduledata runtime.firstmoduledata
var firstmoduledata moduledata

type functab struct {
	entry   uintptr
	funcoff uintptr
}

// findfunctab is an array of these structures.
// Each bucket represents 4096 bytes of the text segment.
// Each subbucket represents 256 bytes of the text segment.
// To find a function given a pc, locate the bucket and subbucket for
// that pc. Add together the idx and subbucket value to obtain a
// function index. Then scan the functab array starting at that
// index to find the target function.
// This table uses 20 bytes for every 4096 bytes of code, or ~0.5% overhead.
type findfuncbucket struct {
	idx        uint32
	subbuckets [16]byte
}

// Mapping information for secondary text sections
type textsect struct {
	vaddr    uintptr // prelinked section vaddr
	length   uintptr // section length
	baseaddr uintptr // relocated section address
}

type nameOff int32
type typeOff int32
type textOff int32

// A ptabEntry is generated by the compiler for each exported function
// and global variable in the main package of a plugin. It is used to
// initialize the plugin module's symbol map.
type ptabEntry struct {
	name nameOff
	typ  typeOff
}

type modulehash struct {
	modulename   string
	linktimehash string
	runtimehash  *string
}

type bitvector struct {
	n        int32 // # of bits
	bytedata *uint8
}

type stackmap struct {
	n        int32   // number of bitmaps
	nbit     int32   // number of bits in each bitmap
	bytedata [1]byte // bitmaps, each starting on a byte boundary
}

type funcInfo struct {
	*_func
	datap *moduledata
}

const minfunc = 16                 // minimum function size
const pcbucketsize = 256 * minfunc // size of bucket in the pc->func lookup table
const nsub = len(findfuncbucket{}.subbuckets)

//go:linkname step runtime.step
func step(p []byte, pc *uintptr, val *int32, first bool) (newp []byte, ok bool)

//go:linkname findfunc runtime.findfunc
func findfunc(pc uintptr) funcInfo

//go:linkname funcdata runtime.funcdata
func funcdata(f funcInfo, i int32) unsafe.Pointer

//go:linkname funcname runtime.funcname
func funcname(f funcInfo) string

//go:linkname moduledataverify1 runtime.moduledataverify1
func moduledataverify1(datap *moduledata)

//ourself defined struct
type funcData struct {
	_func
	Func        *goobj.Func
	stkobjReloc []goobj.Reloc
	Name        string
}

type module struct {
	pclntable []byte
	pcfunc    []findfuncbucket
	funcdata  []funcData
	filetab   []uint32
}

func readFuncData(reloc *CodeReloc, symName string, objSymMap map[string]objSym, codeLen int) (err error) {
	module := &reloc.module
	fd := readAtSeeker{ReadSeeker: objSymMap[symName].file}
	symbol := objSymMap[symName].sym

	x := codeLen
	b := x / pcbucketsize
	i := x % pcbucketsize / (pcbucketsize / nsub)
	for lb := b - len(module.pcfunc); lb >= 0; lb-- {
		module.pcfunc = append(module.pcfunc, findfuncbucket{
			idx: uint32(256 * len(module.pcfunc))})
	}
	bucket := &module.pcfunc[b]
	bucket.subbuckets[i] = byte(len(module.funcdata) - int(bucket.idx))

	pcFileHead := make([]byte, 32)
	pcFileHeadSize := binary.PutUvarint(pcFileHead, uint64(len(module.filetab))<<1)
	for _, fileName := range symbol.Func.File {
		fileName = strings.TrimLeft(fileName, FILE_SYM_PREFIX)
		if offset, ok := reloc.fileMap[fileName]; !ok {
			module.filetab = append(module.filetab, (uint32)(len(module.pclntable)))
			reloc.fileMap[fileName] = len(module.pclntable)
			module.pclntable = append(module.pclntable, []byte(fileName)...)
			module.pclntable = append(module.pclntable, ZERO_BYTE)
		} else {
			module.filetab = append(module.filetab, uint32(offset))
		}
	}

	nameOff := len(module.pclntable)
	module.pclntable = append(module.pclntable, []byte(symbol.Name)...)
	module.pclntable = append(module.pclntable, ZERO_BYTE)

	pcspOff := len(module.pclntable)
	fd.ReadAtWithSize(&(module.pclntable), symbol.Func.PCSP.Size, symbol.Func.PCSP.Offset)

	pcfileOff := len(module.pclntable)
	module.pclntable = append(module.pclntable, pcFileHead[:pcFileHeadSize-1]...)
	fd.ReadAtWithSize(&(module.pclntable), symbol.Func.PCFile.Size, symbol.Func.PCFile.Offset)

	pclnOff := len(module.pclntable)
	fd.ReadAtWithSize(&(module.pclntable), symbol.Func.PCLine.Size, symbol.Func.PCLine.Offset)

	funcdata := funcData{}
	funcdata._func = init_func(symbol, reloc.symMap[symName].Offset, nameOff, pcspOff, pcfileOff, pclnOff)
	for _, data := range symbol.Func.PCData {
		fd.ReadAtWithSize(&(module.pclntable), data.Size, data.Offset)
	}

	for _, data := range symbol.Func.FuncData {
		if _, ok := reloc.stkmaps[data.Sym.Name]; !ok {
			if gcobj, ok := objSymMap[data.Sym.Name]; ok {
				reloc.stkmaps[data.Sym.Name] = make([]byte, gcobj.sym.Data.Size)
				fd := readAtSeeker{ReadSeeker: gcobj.file}
				fd.ReadAt(reloc.stkmaps[data.Sym.Name], gcobj.sym.Data.Offset)
			} else if len(data.Sym.Name) == 0 {
				reloc.stkmaps[data.Sym.Name] = nil
			} else {
				err = errors.New("unknown gcobj:" + data.Sym.Name)
			}
		}
		if strings.Contains(data.Sym.Name, STKOBJ_SUFFIX) {
			funcdata.stkobjReloc = objSymMap[data.Sym.Name].sym.Reloc
		}
	}
	funcdata.Func = symbol.Func
	funcdata.Name = symbol.Name

	module.funcdata = append(module.funcdata, funcdata)

	for _, data := range symbol.Func.FuncData {
		if _, ok := objSymMap[data.Sym.Name]; ok {
			relocSym(reloc, data.Sym.Name, objSymMap)
		}
	}
	return
}

func addModule(codeModule *CodeModule, aModule *moduledata) {
	modules[aModule] = true
	for datap := &firstmoduledata; ; {
		if datap.next == nil {
			datap.next = aModule
			break
		}
		datap = datap.next
	}
	codeModule.module = aModule
}

func removeModule(module interface{}) {
	prevp := &firstmoduledata
	for datap := &firstmoduledata; datap != nil; {
		if datap == module {
			prevp.next = datap.next
			break
		}
		prevp = datap
		datap = datap.next
	}
	delete(modules, module)
}
