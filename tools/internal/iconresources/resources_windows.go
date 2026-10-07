package iconresources

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

func loadGroups(path string) ([]resourceGroup, error) {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	path16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	module, _, err := kernel.NewProc("LoadLibraryExW").Call(uintptr(unsafe.Pointer(path16)), 0, 0x2|0x20)
	if module == 0 {
		return nil, err
	}
	defer kernel.NewProc("FreeLibrary").Call(module)
	// Copy through the OS API so resource addresses never become Go pointers.
	memory := func(address uintptr, length int) ([]byte, error) {
		b := make([]byte, length)
		if length == 0 {
			return b, nil
		}
		ok, _, err := kernel.NewProc("ReadProcessMemory").Call(^uintptr(0), address, uintptr(unsafe.Pointer(&b[0])), uintptr(length), 0)
		if ok == 0 {
			return nil, err
		}
		return b, nil
	}
	read := func(kind, name, language uintptr) ([]byte, error) {
		resource, _, err := kernel.NewProc("FindResourceExW").Call(module, kind, name, language)
		if resource == 0 {
			return nil, err
		}
		length, _, err := kernel.NewProc("SizeofResource").Call(module, resource)
		if length == 0 {
			return nil, err
		}
		handle, _, err := kernel.NewProc("LoadResource").Call(module, resource)
		if handle == 0 {
			return nil, err
		}
		address, _, err := kernel.NewProc("LockResource").Call(handle)
		if address == 0 {
			return nil, err
		}
		return memory(address, int(length))
	}
	var groups []resourceGroup
	var callbackErr error
	// Create callbacks once per enumeration, not once per group or language.
	var groupName any
	languagesCallback := syscall.NewCallback(func(_module, _kind, name, language, _context uintptr) uintptr {
		data, err := read(14, name, language)
		if err != nil {
			callbackErr = err
			return 0
		}
		entries, err := groupEntries(data)
		if err != nil {
			callbackErr = err
			return 0
		}
		group := resourceGroup{Name: groupName, Language: uint16(language), Data: data, Icons: map[uint16][]byte{}}
		for _, entry := range entries {
			id := binary.LittleEndian.Uint16(entry[12:])
			payload, err := read(3, uintptr(id), language)
			if err != nil {
				callbackErr = err
				return 0
			}
			group.Icons[id] = payload
		}
		groups = append(groups, group)
		return 1
	})
	namesCallback := syscall.NewCallback(func(_module, _kind, name, _context uintptr) uintptr {
		if name <= 65535 {
			groupName = int(name)
		} else {
			var units []uint16
			for address := name; ; address += 2 {
				b, err := memory(address, 2)
				if err != nil {
					callbackErr = err
					return 0
				}
				unit := binary.LittleEndian.Uint16(b)
				if unit == 0 {
					break
				}
				units = append(units, unit)
			}
			groupName = string(utf16.Decode(units))
		}
		ok, _, err := kernel.NewProc("EnumResourceLanguagesW").Call(module, 14, name, languagesCallback, 0)
		if ok == 0 {
			if callbackErr == nil {
				callbackErr = err
			}
			return 0
		}
		return 1
	})
	ok, _, err := kernel.NewProc("EnumResourceNamesW").Call(module, 14, namesCallback, 0)
	runtime.KeepAlive(path16)
	if callbackErr != nil {
		return nil, callbackErr
	}
	if ok == 0 {
		return nil, fmt.Errorf("EnumResourceNamesW: %w", err)
	}
	return groups, nil
}
