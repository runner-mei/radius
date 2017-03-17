package radius

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sync"
	//	"io"
	"bytes"
	"encoding/binary"
	"path/filepath"
	"strconv"
	"strings"
)

var builtinOnce sync.Once

// Builtin is the built-in dictionary. It is initially loaded with the
// attributes defined in RFC 2865 and RFC 2866.
var Builtin *Dictionary

func initDictionary() {
	Builtin = &Dictionary{}
	Builtin.RegisterVendor("default", 1)
}

type dictEntry struct {
	Type  byte
	Name  string
	Codec AttributeCodec
}

type dictAttr struct {
	attributesByType [1069]*dictEntry
	attributesByName map[string]*dictEntry
}

// Dictionary stores mappings between attribute names and types and
// AttributeCodecs.
type Dictionary struct {
	mu       sync.RWMutex
	Vendor   string
	VendorId map[string]int // initied to zero,so vendor must above zero
	values   map[string]*dictAttr
}

func (d *Dictionary) to_byte(bst string) byte {
	i, _ := strconv.ParseInt(bst, 10, 8)
	//fmt.Println(i)
	b_buf := bytes.NewBuffer([]byte{})
	// intel x86 is littleEndian
	binary.Write(b_buf, binary.LittleEndian, i)
	return b_buf.Bytes()[0]
}

func (d *Dictionary) ParseAttrs(arr []string) bool {
	if len(arr) != 4 {
		return false
	}
	if strings.ToUpper(arr[0]) == "ATTRIBUTE" {
		num, _ := strconv.ParseInt(arr[2], 10, 32)
		if num > 255 {
			return false
		}
		//		fmt.Println(arr)
		switch arr[3] {
		case "string":
			d.MustRegister(arr[1], d.to_byte(arr[2]), AttributeString)
		case "integer":
			d.MustRegister(arr[1], d.to_byte(arr[2]), AttributeInteger)
		case "ipaddr":
			d.MustRegister(arr[1], d.to_byte(arr[2]), AttributeAddress)
		case "octets":
			d.MustRegister(arr[1], d.to_byte(arr[2]), AttributeString)
		case "date":
			d.MustRegister(arr[1], d.to_byte(arr[2]), AttributeTime)
		}
		return true
	}

	return false
}

func (d *Dictionary) ParseVendor(arr []string) bool {
	if len(arr) != 3 {
		return false
	}

	if strings.ToUpper(arr[0]) == "VENDOR" {
		vid, _ := strconv.Atoi(arr[2])
		if vid <= 0 {
			panic("ParseVendor ID <= 0 ")
			return false
		}
		d.RegisterVendor(arr[1], vid)
		return true
	}

	return false
}

func (d *Dictionary) ParseBeginVendor(arr []string) bool {
	if len(arr) != 2 {
		return false
	}

	if strings.ToUpper(arr[0]) == "BEGIN-VENDOR" {
		d.SwitchVendor(arr[1])
		return true
	}

	return false

}

func (d *Dictionary) ParseEndVendor(arr []string) bool {
	if len(arr) != 2 {
		return false
	}
	if strings.ToUpper(arr[0]) == "END-VENDOR" {

		d.SwitchVendor("default")
		return true
	}

	return false
}

func (d *Dictionary) LoadDicts(path string) {
	dir, err := filepath.Abs(filepath.Dir(path))
	if err == nil {
		fmt.Println(dir)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println(path, " not existed")
		return
	}
	inFile, _ := os.Open(path)
	defer inFile.Close()
	scanner := bufio.NewScanner(inFile)
	//	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		arr := strings.Fields(line)

		if d.ParseAttrs(arr) != true {
			if d.ParseVendor(arr) != true {
				if d.ParseBeginVendor(arr) != true {
					d.ParseEndVendor(arr)
				}
			}
		}
	}
}

func (d *Dictionary) Values() *dictAttr {
	return d.values[d.Vendor]
}

func (d *Dictionary) GetVendorId(v string) int {
	return d.VendorId[v]
}

func (d *Dictionary) SwitchVendor(v string) {
	if d.VendorId[v] != 0 {
		d.Vendor = v
	}
}

func (d *Dictionary) RegisterVendor(v string, id int) {
	if id <= 0 {
		panic("RegisterVendor ID must > 0")
		return
	}
	if d.VendorId == nil {
		d.VendorId = make(map[string]int)
	}

	if d.VendorId[v] == 0 {
		d.Vendor = v
		d.VendorId[v] = id
		if d.values == nil {
			d.values = make(map[string]*dictAttr)
		}
		if d.values[d.Vendor] == nil {
			d.values[d.Vendor] = &dictAttr{}
		}
	}
}

// Register registers the AttributeCodec for the given attribute name and type.
func (d *Dictionary) Register(name string, t byte, codec AttributeCodec) error {
	d.mu.Lock()
	if d.values == nil {
		d.values = make(map[string]*dictAttr)
	}
	if d.values[d.Vendor] == nil {
		d.values[d.Vendor] = &dictAttr{}
	}
	if d.VendorId == nil {
		d.VendorId = make(map[string]int)
	}
	if d.Values().attributesByType[t] != nil {
		d.mu.Unlock()
		return errors.New("radius: attribute already registered")
	}
	entry := &dictEntry{
		Type:  t,
		Name:  name,
		Codec: codec,
	}
	d.Values().attributesByType[t] = entry
	if d.Values().attributesByName == nil {
		d.Values().attributesByName = make(map[string]*dictEntry)
	}

	d.Values().attributesByName[name] = entry
	d.mu.Unlock()
	return nil
}

// MustRegister is a helper for Register that panics if it returns an error.
func (d *Dictionary) MustRegister(name string, t byte, codec AttributeCodec) {
	if err := d.Register(name, t, codec); err != nil {
		panic(err)
	}
}

func (d *Dictionary) get(name string) (t byte, codec AttributeCodec, ok bool) {
	d.mu.RLock()
	entry := d.Values().attributesByName[name]
	d.mu.RUnlock()
	if entry == nil {
		return
	}
	t = entry.Type
	codec = entry.Codec
	ok = true
	return
}

// Attr returns a new *Attribute whose type is registered under the given
// name.
//
// If name is not registered, nil and an error is returned.
//
// If the attribute's codec implements AttributeTransformer, the value is
// first transformed before being stored in *Attribute. If the transform
// function returns an error, nil and the error is returned.
func (d *Dictionary) Attr(name string, value interface{}) (*Attribute, error) {
	t, codec, ok := d.get(name)
	if !ok {
		return nil, errors.New("radius: attribute name not registered")
	}
	if transformer, ok := codec.(AttributeTransformer); ok {
		transformed, err := transformer.Transform(value)
		if err != nil {
			return nil, err
		}
		value = transformed
	}
	return &Attribute{
		Type:  t,
		Value: value,
	}, nil
}

// MustAttr is a helper for Attr that panics if Attr were to return an error.
func (d *Dictionary) MustAttr(name string, value interface{}) *Attribute {
	attr, err := d.Attr(name, value)
	if err != nil {
		panic(err)
	}
	return attr
}

// Name returns the registered name for the given attribute type. ok is false
// if the given type is not registered.
func (d *Dictionary) Name(t byte) (name string, ok bool) {
	d.mu.RLock()
	entry := d.Values().attributesByType[t]
	d.mu.RUnlock()
	if entry == nil {
		return
	}
	name = entry.Name
	ok = true
	return
}

// Type returns the registered type for the given attribute name. ok is false
// if the given name is not registered.
func (d *Dictionary) Type(name string) (t byte, ok bool) {
	d.mu.RLock()
	entry := d.Values().attributesByName[name]
	d.mu.RUnlock()
	if entry == nil {
		return
	}
	t = entry.Type
	ok = true
	return
}

// Codec returns the AttributeCodec for the given registered type. nil is
// returned if the given type is not registered.
func (d *Dictionary) Codec(t byte) AttributeCodec {
	d.mu.RLock()
	entry := d.Values().attributesByType[t]
	d.mu.RUnlock()
	if entry == nil {
		return AttributeUnknown
	}
	return entry.Codec
}
