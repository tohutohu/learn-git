package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"
	"syscall"

	"github.com/urfave/cli"
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

	app := cli.NewApp()
	app.Commands = []cli.Command{
		{
			Name:    "status",
			Aliases: []string{"s"},
			Usage:   "show status",
			Action: func(ctx *cli.Context) error {
				body, _ := ioutil.ReadFile(".git/index")
				idx, err := parseIndex(body)

				fmt.Printf("%+v\n", idx)
				return err
			},
		},
		{
			Name: "commit",
			Action: func(ctx *cli.Context) error {
				// 対象のファイルリストを取得する
				fileList := dirwalk(".")
				fmt.Println(fileList)

				_, err := createCommitObject(fileList)
				if err != nil {
					panic(err)
				}

				body, _ := ioutil.ReadFile(".git/index")
				idx, err := parseIndex(body)

				fmt.Printf("%+v\n", idx)

				updateIndex()

				body, _ = ioutil.ReadFile(".git/index")
				idx, err = parseIndex(body)

				fmt.Printf("%+v\n", idx)
				return err
			},
		},
		{
			Name: "sha",
			Action: func(ctx *cli.Context) error {
				if len(ctx.Args()) != 1 {
					return errors.New("FimeName plz")
				}

				hash, err := getFileHash(ctx.Args()[0])
				if err != nil {
					return err
				}
				fmt.Println(hash)
				return nil
			},
		},
		{
			Name: "update",
			Action: func(ctx *cli.Context) error {
				if err := updateIndex(); err != nil {
					return err
				}
				return nil
			},
		},
		{
			Name: "index-byte",
			Action: func(ctx *cli.Context) error {
				body, _ := ioutil.ReadFile(".git/index")
				fmt.Println(body)
				return nil
			},
		},
	}

	sort.Sort(cli.FlagsByName(app.Flags))
	sort.Sort(cli.CommandsByName(app.Commands))

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func updateIndex() error {
	idx := &index{}
	idx.version = 2

	fileList := dirwalk(".")
	for _, fileName := range fileList {
		fmt.Println(fileName)
		stat, err := os.Stat(fileName)
		if err != nil {
			return err
		}
		fileInfo, ok := stat.Sys().(*syscall.Stat_t)
		if !ok {
			return errors.New("cant cast")
		}

		newEntry := entry{}
		newEntry.ctime = uint32(fileInfo.Ctimespec.Sec)
		newEntry.ctimeNano = uint32(fileInfo.Ctimespec.Nsec)

		newEntry.mTime = uint32(fileInfo.Mtimespec.Sec)
		newEntry.mTimeNano = uint32(fileInfo.Mtimespec.Nsec)

		newEntry.dev = uint32(fileInfo.Dev)

		newEntry.inode = uint32(fileInfo.Ino)

		newEntry.mode = stat.Mode()
		newEntry.uid = fileInfo.Uid
		newEntry.guid = fileInfo.Gid

		newEntry.size = uint32(stat.Size())
		hash, err := getFileHash(fileName)
		if err != nil {
			return err
		}
		newEntry.sha1 = hash
		newEntry.assumeValid = false
		newEntry.extendedFlag = false
		newEntry.nameLen = uint16(len(fileName))
		newEntry.name = fileName
		idx.entries = append(idx.entries, newEntry)
		idx.entryCount++
	}
	sort.Slice(idx.entries, func(i, j int) bool {
		return bytes.Compare([]byte(idx.entries[i].name), []byte(idx.entries[j].name)) <= 0
	})
	fmt.Printf("%+v\n", idx)

	buf, err := idx.getBytes()
	if err != nil {
		return err
	}
	h := sha1.New()
	h.Write(buf.Bytes())

	checkSum := h.Sum(nil)
	buf.Write(checkSum)

	fmt.Println(buf.Bytes())
	if err := ioutil.WriteFile(gitDir+"/index", buf.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}

func (idx *index) getBytes() (bytes.Buffer, error) {
	var buf bytes.Buffer
	buf.WriteString("DIRC")
	versionBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(versionBytes, idx.version)

	countBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(countBytes, idx.entryCount)

	buf.Write(versionBytes)
	buf.Write(countBytes)
	for _, en := range idx.entries {

		entryBuf, err := en.getBytes()
		if err != nil {
			return buf, err
		}
		buf.Write(entryBuf.Bytes())
	}
	return buf, nil
}

func (en *entry) getBytes() (bytes.Buffer, error) {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, en.ctime)
	binary.Write(&buf, binary.BigEndian, en.ctimeNano)
	binary.Write(&buf, binary.BigEndian, en.mTime)
	binary.Write(&buf, binary.BigEndian, en.mTimeNano)
	binary.Write(&buf, binary.BigEndian, en.dev)
	binary.Write(&buf, binary.BigEndian, en.inode)
	mode := make([]byte, 4)
	binary.BigEndian.PutUint32(mode, uint32(en.mode))
	mode[2] |= 0x80
	buf.Write(mode)

	binary.Write(&buf, binary.BigEndian, en.uid)
	binary.Write(&buf, binary.BigEndian, en.guid)
	binary.Write(&buf, binary.BigEndian, en.size)
	sha1Bytes, err := hex.DecodeString(en.sha1)
	if err != nil {
		return buf, err
	}
	buf.Write(sha1Bytes)

	flags := make([]byte, 2)
	flags[0] |= boolToByte(en.extendedFlag) << 7
	flags[0] |= boolToByte(en.assumeValid) << 6
	flags[0] |= en.stage << 4
	nameLen := make([]byte, 2)
	binary.BigEndian.PutUint16(nameLen, en.nameLen)
	flags[0] |= (nameLen[0] & 0x0f)
	flags[1] = nameLen[1]
	buf.Write(flags)

	buf.WriteString(en.name)

	paddingNum := 10 - (en.nameLen % 10)
	if paddingNum == 0 {
		paddingNum = 10
	}
	padding := make([]byte, paddingNum)
	buf.Write(padding)

	return buf, nil
}

func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
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

		en.extendedFlag = 0 != (flags & 0x8000)
		en.assumeValid = 0 != (flags & 0x4000)
		en.stage = uint8((flags & 0x3000) >> 12)

		en.nameLen = flags & 0x0fff

		name, entryBody := entryBody[:en.nameLen], entryBody[en.nameLen:]
		fmt.Println("name: ", hex.EncodeToString(name))
		fmt.Println("name: ", string(name))
		fmt.Println("name length: ", en.nameLen)
		en.name = string(name)

		paddingNum := 10 - (en.nameLen % 10)
		if paddingNum == 0 {
			paddingNum = 10
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
	return strings.Trim(string(buf), "\n"), nil
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
