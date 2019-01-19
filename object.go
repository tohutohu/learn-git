package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"time"
)

func createBlobObject(fileName string) (string, error) {
	fileStat, err := os.Stat(fileName)
	if err != nil {
		fmt.Println("file not exists")
		return "", err
	}

	if fileStat.IsDir() {
		fmt.Printf("%s is directory\n", fileName)
		return "", err
	}

	header := []byte("blob " + strconv.Itoa(int(fileStat.Size())) + "\u0000")

	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		fmt.Println("file read error")
		return "", err
	}
	hash, err := createObject(header, content)
	fmt.Println("blob: ", hash)
	return hash, err
}

func createTreeObject(fileNames []string) (string, error) {
	content := bytes.Buffer{}
	for _, fileName := range fileNames {
		hash, err := createBlobObject(fileName)
		if err != nil {
			return "", err
		}

		fileInfo, _ := os.Stat(fileName)

		content.WriteString("100")
		content.WriteString(getPermission(fileInfo.Mode()))
		content.WriteString(" ")
		content.WriteString(fileName)
		content.WriteString("\u0000")
		data, _ := hex.DecodeString(hash)
		content.Write(data)
	}
	header := []byte("tree " + strconv.Itoa(len(content.String())) + "\u0000")

	hash, err := createObject(header, content.Bytes())
	fmt.Println("tree: ", hash)
	return hash, err
}

func createCommitObject(fileNames []string) (string, error) {
	content := bytes.Buffer{}

	hash, err := createTreeObject(fileNames)
	if err != nil {
		return "", err
	}

	content.WriteString("tree ")
	content.WriteString(hash)
	content.WriteString("\n")
	parent, err := getHeadHash()
	if err == nil {
		content.WriteString("parent ")
		content.WriteString(parent)
		content.WriteString("\n")
	} else {
		fmt.Println("error: ", err)
	}
	content.WriteString("author to-hutohu <tohu.soy@gmail.com> ")
	content.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
	content.WriteString(" +0900\n")
	content.WriteString("committer to-hutohu <tohu.soy@gmail.com> ")
	content.WriteString(strconv.FormatInt(time.Now().Unix(), 10))
	content.WriteString(" +0900\n\n")
	content.WriteString("commited!!!!!")

	header := []byte("commit " + strconv.Itoa(len(content.String())) + "\u0000")
	hash, err = createObject(header, content.Bytes())
	fmt.Println("commit: ", hash)
	if err != nil {
		return hash, err
	}
	fmt.Println("update ref")
	err = updateRefs(hash)
	return hash, err
}

func createObject(header, content []byte) (string, error) {
	h := sha1.New()
	h.Write(append(header, content...))

	hash := hex.EncodeToString(h.Sum(nil))

	objectsDir := gitDir + "/objects/"

	dirName := objectsDir + hash[:2] + "/"
	objName := dirName + hash[2:]

	os.MkdirAll(dirName, 0766)

	file, err := os.Create(objName)
	if err != nil {
		fmt.Println("file create error")
		return "", err
	}
	defer file.Close()
	var s bytes.Buffer

	z, _ := zlib.NewWriterLevel(&s, 9)
	z.Write(append(header, content...))
	z.Flush()
	z.Close()
	file.Write(s.Bytes())

	return hash, nil
}
