package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"
)

type entry struct {
	ctime        uint32
	ctimeNano    uint32
	mTime        uint32
	mTimeNano    uint32
	dev          uint32
	inode        uint32
	mode         os.FileMode
	uid          uint32
	guid         uint32
	size         uint32
	sha1         string
	assumeValid  bool
	extendedFlag bool
	stage        uint8
	nameLen      uint16
	name         string
}

type index struct {
	version    uint32
	entryCount uint32
	entries    []entry
}

var (
	gitDir = ".git"
)

func main() {
	flag.Parse()
	createCommitObject([]string{"main.go", "readme"})
	// body, _ := ioutil.ReadFile(".git/index")
	// idx, err := parseIndex(body)
	// fmt.Println(err)
	// fmt.Printf("%+v", idx)
}

func parseIndex(body []byte) (*index, error) {
	idx := &index{}

	// signature
	signature, body := body[:4], body[4:]
	if string(signature) != "DIRC" {
		fmt.Println("wrong signature")
		return nil, errors.New("wrong signature")
	}

	byteVersion, body := body[:4], body[4:]
	version := binary.BigEndian.Uint32(byteVersion)
	idx.version = version

	entryCount, body := body[:4], body[4:]
	idx.entryCount = binary.BigEndian.Uint32(entryCount)

	for i := uint32(0); i < idx.entryCount; i++ {
		fmt.Println("==================== entry start ===================")
		en := entry{}
		ctime, entryBody := body[:4], body[4:]
		fmt.Println("ctime: ", hex.EncodeToString(ctime))
		en.ctime = binary.BigEndian.Uint32(ctime)

		ctimeNano, entryBody := entryBody[:4], entryBody[4:]
		fmt.Println("ctimeNano: ", hex.EncodeToString(ctimeNano))
		en.ctimeNano = binary.BigEndian.Uint32(ctimeNano)

		mTime, entryBody := entryBody[:4], entryBody[4:]
		fmt.Println("mTime: ", hex.EncodeToString(mTime))
		en.mTime = binary.BigEndian.Uint32(mTime)

		mTimeNano, entryBody := entryBody[:4], entryBody[4:]
		fmt.Println("mTimeNano: ", hex.EncodeToString(mTimeNano))
		en.mTimeNano = binary.BigEndian.Uint32(mTimeNano)

		dev, entryBody := entryBody[:4], entryBody[4:]
		fmt.Println("dev: ", hex.EncodeToString(dev))
		en.dev = binary.BigEndian.Uint32(dev)

		inode, entryBody := entryBody[:4], entryBody[4:]
		fmt.Println("inode: ", hex.EncodeToString(inode))
		en.inode = binary.BigEndian.Uint32(inode)

		mode, entryBody := entryBody[:4], entryBody[4:]
		fmt.Println("mode: ", hex.EncodeToString(mode))
		en.mode = os.FileMode(binary.BigEndian.Uint32(mode))

		uid, entryBody := entryBody[:4], entryBody[4:]
		fmt.Println("uid: ", hex.EncodeToString(uid))
		en.uid = binary.BigEndian.Uint32(uid)

		guid, entryBody := entryBody[:4], entryBody[4:]
		fmt.Println("guid: ", hex.EncodeToString(guid))
		en.guid = binary.BigEndian.Uint32(guid)

		size, entryBody := entryBody[:4], entryBody[4:]
		fmt.Println("size: ", hex.EncodeToString(size))
		en.size = binary.BigEndian.Uint32(size)

		sha, entryBody := entryBody[:20], entryBody[20:]
		fmt.Println("sha: ", hex.EncodeToString(sha))
		en.sha1 = hex.EncodeToString(sha)

		flagsByte, entryBody := entryBody[:2], entryBody[2:]
		fmt.Println("flags: ", hex.EncodeToString(flagsByte))
		flags := binary.BigEndian.Uint16(flagsByte)

		en.assumeValid = 0 != (flags & 0x4000)

		en.extendedFlag = 0 != (flags & 0x8000)

		en.stage = uint8((flags & 0x3000) >> 12)

		en.nameLen = flags & 0x0fff

		name, entryBody := entryBody[:en.nameLen], entryBody[en.nameLen:]
		fmt.Println("name: ", hex.EncodeToString(name))
		fmt.Println("name: ", string(name))
		en.name = string(name)

		paddingNum := (en.nameLen % 4)
		if paddingNum == 0 {
			paddingNum = 4
		}
		fmt.Println("padding num:", paddingNum)
		padding, entryBody := entryBody[:paddingNum], entryBody[paddingNum:]
		fmt.Println("padding: ", hex.EncodeToString(padding))

		idx.entries = append(idx.entries, en)
		body = entryBody
		fmt.Println("==================== entry end ===================\n\n")
	}

	return idx, nil
}

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

func getPermission(m os.FileMode) string {
	num := uint32(m)
	p1 := strconv.Itoa(int((num & 448) >> 6))
	p2 := strconv.Itoa(int((num & 56) >> 3))
	p3 := strconv.Itoa(int((num & 7)))
	return p1 + p2 + p3
}

func getHeadHash() (string, error) {
	head, err := getHead()
	if err != nil {
		return "", err
	}

}

func getHead() (string, error) {
	byteBody, err := ioutil.ReadFile(gitDir + "/HEAD")
	body := string(byteBody)
	ref := strings.Split(body, " ")
	if len(ref) != 2 {
		return "", errors.New("HEAD File is invalid")
	}

	return ref[1], nil
}
