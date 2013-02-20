package ws

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"
)

const (
	FlagDir uint32 = 1 << iota
	FlagLogical
	FlagMount
)

type Res struct {
	Name   string
	Flag   uint32
	Parent *Res
	*Dir
	sync.Mutex
}

type Dir struct {
	Path     string
	Children []*Res
}

func (r *Res) Path() string {
	if r == nil {
		return ""
	}
	if r.Dir != nil {
		return r.Dir.Path
	}
	return r.Parent.Path() + string(os.PathSeparator) + r.Name
}

type Ws struct {
	sync.RWMutex
	root *Res
	all  map[string]*Res
}

func New() *Ws {
	r := &Res{}
	m := make(map[string]*Res, 10000)
	m["/"] = r
	return &Ws{root: r, all: m}
}
func (w *Ws) addParent(path string) *Res {
	if r, ok := w.all[path]; ok {
		return r
	}
	parent, name := filepath.Split(path)
	var pa *Res
	if len(parent) > 0 && parent != "/" {
		pa = w.addParent(parent[:len(parent)-1])
	}
	r := &Res{Name: name, Flag: FlagDir | FlagLogical, Parent: pa}
	w.all[path] = r
	return r
}
func newChild(pa *Res, fi os.FileInfo) *Res {
	r := &Res{Name: fi.Name(), Parent: pa}
	if fi.IsDir() {
		r.Flag |= FlagDir
		r.Dir = &Dir{Path: r.Path()}
	}
	return r
}
func read(r *Res) error {
	f, err := os.Open(r.Dir.Path)
	if err != nil {
		return err
	}
	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return err
	}
	children := make([]*Res, 0, len(list))
	for _, fi := range list {
		children = append(children, newChild(r, fi))
	}
	sort.Sort(byTypeAndName(children))
	r.Children = children
	for _, c := range children {
		if c.Flag&FlagDir != 0 {
			if err := read(c); err != nil {
				fmt.Println(err)
			}
		}
	}
	return nil
}

type byTypeAndName []*Res

func (l byTypeAndName) Len() int {
	return len(l)
}

func (l byTypeAndName) Less(i, j int) bool {
	if isdir := l[i].Flag&FlagDir != 0; isdir != (l[j].Flag&FlagDir != 0) {
		return isdir
	}
	return l[i].Name < l[j].Name
}

func (l byTypeAndName) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (w *Ws) addAllChildren(r *Res) {
	for _, c := range r.Children {
		w.all[c.Path()] = c
		if c.Flag&FlagDir != 0 {
			w.addAllChildren(c)
		}
	}
}
func (w *Ws) Mount(path string) (*Res, error) {
	w.RLock()
	r, ok := w.all[path]
	w.RUnlock()
	if ok {
		return r, nil
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}
	d, f := filepath.Split(path)
	// add virtual parent
	r = &Res{Name: f, Flag: FlagDir | FlagMount, Dir: &Dir{Path: path}}
	err = read(r)
	if err != nil {
		return nil, err
	}
	w.Lock()
	defer w.Unlock()
	r.Parent = w.addParent(d[:len(d)-1])
	w.all[path] = r
	w.addAllChildren(r)
	return r, nil
}
func mountAllSeq(w *Ws, dirs []string) {
	for _, path := range dirs {
		_, err := w.Mount(path)
		if err != nil {
			fmt.Println(err)
		}
	}
}
func mountAllPar(w *Ws, dirs []string) {
	var wg sync.WaitGroup
	wg.Add(len(dirs))
	for _, path := range dirs {
		go func(path string, wg *sync.WaitGroup) {
			_, err := w.Mount(path)
			if err != nil {
				fmt.Println(err)
			}
			wg.Done()
		}(path, &wg)
	}
	wg.Wait()
}
func TestWalkSrc(t *testing.T) {
	dirs := build.Default.SrcDirs()
	t.Log(dirs)
	w := New()
	start := time.Now()
	if runtime.GOMAXPROCS(0) > 1 {
		mountAllPar(w, dirs)
	} else {
		mountAllSeq(w, dirs)
	}
	for p, r := range w.all {
		if p != r.Path() {
			t.Error(p, "!=", r.Path())
		}
	}
	took := time.Since(start)
	runtime.GC()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	f := "count: %d, took: %s, alloc: %d/%d kb, heap: %d/%d kb, objs: %d, gcs: %d"
	kb := func(n uint64) uint64 { return n / (1 << 10) }
	t.Logf(f, len(w.all), took, kb(mem.Alloc), kb(mem.TotalAlloc), kb(mem.HeapAlloc), kb(mem.HeapSys), mem.HeapObjects, mem.NumGC)
}
