//go:build windows

package winsys

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const hkeyCurrentUser = 0x80000001

var (
	advapi32                    = windows.NewLazySystemDLL("advapi32.dll")
	ktmw32                      = windows.NewLazySystemDLL("ktmw32.dll")
	procRegCreateKeyTransactedW = advapi32.NewProc("RegCreateKeyTransactedW")
	procRegSetValueExW          = advapi32.NewProc("RegSetValueExW")
	procCreateTransaction       = ktmw32.NewProc("CreateTransaction")
	procCommitTransaction       = ktmw32.NewProc("CommitTransaction")
	procRollbackTransaction     = ktmw32.NewProc("RollbackTransaction")
	errRegistryTxNotAvailable   = errors.New("registry transactions are not available")
)

// writeThemeRegistry commits every value in one kernel transaction. If
// transactions are unavailable (for example on an older or stripped-down
// Windows image), it falls back to the regular registry API.
func writeThemeRegistry(values []registryValue) error {
	if len(values) == 0 {
		return nil
	}
	err := writeRegistryTransaction(values)
	if err == nil {
		return nil
	}
	if !errors.Is(err, errRegistryTxNotAvailable) {
		return err
	}
	return writeRegistryDirect(values)
}

func writeRegistryTransaction(values []registryValue) error {
	tx, err := createRegistryTransaction()
	if err != nil {
		return fmt.Errorf("%w: %v", errRegistryTxNotAvailable, err)
	}
	defer windows.CloseHandle(tx)

	committed := false
	defer func() {
		if !committed {
			procRollbackTransaction.Call(uintptr(tx))
		}
	}()

	keys := make(map[string]windows.Handle, 4)
	defer func() {
		for _, key := range keys {
			windows.RegCloseKey(key)
		}
	}()

	for _, value := range values {
		key, ok := keys[value.Path]
		if !ok {
			key, err = createKeyTransacted(value.Path, tx)
			if err != nil {
				return err
			}
			keys[value.Path] = key
		}
		if err := setValueTransacted(key, value); err != nil {
			return err
		}
	}

	// Keys must be closed before the transaction is committed.
	for path, key := range keys {
		windows.RegCloseKey(key)
		delete(keys, path)
	}

	if err := commitRegistryTransaction(tx); err != nil {
		return err
	}
	committed = true
	return nil
}

func createRegistryTransaction() (windows.Handle, error) {
	handle, _, callErr := procCreateTransaction.Call(0, 0, 0, 0, 0, 0, 0)
	if handle == 0 {
		return 0, fmt.Errorf("CreateTransaction: %w", callErr)
	}
	return windows.Handle(handle), nil
}

func createKeyTransacted(path string, tx windows.Handle) (windows.Handle, error) {
	subKey, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var key windows.Handle
	var disposition uint32
	status, _, _ := procRegCreateKeyTransactedW.Call(
		hkeyCurrentUser,
		uintptr(unsafe.Pointer(subKey)),
		0, // Reserved
		0, // Class
		0, // Non-volatile, no special options
		uintptr(registry.SET_VALUE),
		0, // Security attributes
		uintptr(unsafe.Pointer(&key)),
		uintptr(unsafe.Pointer(&disposition)),
		uintptr(tx),
		0, // Extended parameter
	)
	if status != 0 {
		return 0, fmt.Errorf("RegCreateKeyTransactedW %s: %w", path, syscall.Errno(status))
	}
	return key, nil
}

func setValueTransacted(key windows.Handle, value registryValue) error {
	name, err := windows.UTF16PtrFromString(value.Name)
	if err != nil {
		return err
	}

	var data unsafe.Pointer
	var size uint32
	var dword uint32
	switch value.Type {
	case registryDWORD:
		dword = value.DWord
		data = unsafe.Pointer(&dword)
		size = 4
	case registrySZ:
		encoded, err := windows.UTF16FromString(value.String)
		if err != nil {
			return fmt.Errorf("encode %s\\%s: %w", value.Path, value.Name, err)
		}
		data = unsafe.Pointer(&encoded[0])
		size = uint32(len(encoded) * 2)
	default:
		return fmt.Errorf("unsupported registry type %d for %s\\%s", value.Type, value.Path, value.Name)
	}

	status, _, _ := procRegSetValueExW.Call(
		uintptr(key),
		uintptr(unsafe.Pointer(name)),
		0, // Reserved
		uintptr(value.Type),
		uintptr(data),
		uintptr(size),
	)
	if status != 0 {
		return fmt.Errorf("RegSetValueExW %s\\%s: %w", value.Path, value.Name, syscall.Errno(status))
	}
	return nil
}

func commitRegistryTransaction(tx windows.Handle) error {
	ok, _, callErr := procCommitTransaction.Call(uintptr(tx))
	if ok == 0 {
		return fmt.Errorf("CommitTransaction: %w", callErr)
	}
	return nil
}

// writeRegistryDirect is the compatibility path for images where KTM registry
// transactions are disabled. It mirrors the transaction value list.
func writeRegistryDirect(values []registryValue) error {
	for _, value := range values {
		key, _, err := registry.CreateKey(registry.CURRENT_USER, value.Path, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("open %s: %w", value.Path, err)
		}
		switch value.Type {
		case registryDWORD:
			err = key.SetDWordValue(value.Name, value.DWord)
		case registrySZ:
			err = key.SetStringValue(value.Name, value.String)
		default:
			err = fmt.Errorf("unsupported registry type %d", value.Type)
		}
		key.Close()
		if err != nil {
			return fmt.Errorf("set %s\\%s: %w", value.Path, value.Name, err)
		}
	}
	return nil
}
