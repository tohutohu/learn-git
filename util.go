package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

func dirwalk(dir string) []string {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	var paths []string
	for _, file := range files {
		if file.Name() == ".git" {
			continue
		}
		if file.IsDir() {
			paths = append(paths, dirwalk(filepath.Join(dir, file.Name()))...)
			continue
		}
		paths = append(paths, filepath.Join(dir, file.Name()))
	}

	return paths
}

func getPermission(m os.FileMode) string {
	num := uint32(m)
	p1 := strconv.Itoa(int((num & 448) >> 6))
	p2 := strconv.Itoa(int((num & 56) >> 3))
	p3 := strconv.Itoa(int((num & 7)))
	return p1 + p2 + p3
}

func isHash(str string) bool {
	buf, err := hex.DecodeString(str)
	if err != nil {
		return false
	}
	if len(buf) != 20 {
		return false
	}
	return true
}

func updateRefs(commitObjectHash string) error {
	head, err := getHead()
	if err != nil {
		return err
	}
	fmt.Println("write hash", commitObjectHash)
	fmt.Println("filename: ", gitDir+"/"+head, "po")
	if isHash(head) {
		err := ioutil.WriteFile(gitDir+"/HEAD", []byte(commitObjectHash), 0644)
		if err != nil {
			return err
		}
		return nil
	}
	if err := ioutil.WriteFile(gitDir+"/"+head, []byte(commitObjectHash), 0644); err != nil {
		return err
	}
	return nil
}
