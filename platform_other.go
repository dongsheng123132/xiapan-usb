//go:build !windows

package main

func platformInfo(root string) (uint64, uint64, []Volume, []string) {
	return 0, 0, []Volume{}, []string{"This build is supported only on Windows x64."}
}
