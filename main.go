package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
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
	// 対象のファイルリストを取得する
	fileList := dirwalk(".")
	fmt.Println(fileList)

	_, err := createCommitObject(fileList)
	if err != nil {
		panic(err)
	}
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

func getHeadHash() (string, error) {
	head, err := getHead()
	if err != nil {
		return "", err
	}
	if isHash(head) {
		return head, nil
	}
	buf, err := ioutil.ReadFile(gitDir + "/" + head)
	if err != nil {
		return "", err
	}
	fmt.Println("head hash: ", string(buf))
	return string(buf), nil
}

func getHead() (string, error) {
	byteBody, err := ioutil.ReadFile(gitDir + "/HEAD")
	if err != nil {
		return "", err
	}
	body := string(byteBody)
	ref := strings.Split(body, " ")
	if len(ref) != 2 {
		return "", errors.New("HEAD File is invalid")
	}

	return strings.Trim(ref[1], "\n"), nil
}
