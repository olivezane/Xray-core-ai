package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xtls/xray-core/common"
)

var directory = flag.String("pwd", "", "Working directory of Xray vprotogen.")

func whichProtoc(suffix string) (string, error) {
	protoc := "protoc" + suffix

	path, err := exec.LookPath(protoc)
	if err != nil {
		return "", fmt.Errorf(`
Command "%s" not found.
Make sure that %s is in your system path or current path.
Download %s from https://github.com/protocolbuffers/protobuf/releases
`, protoc, protoc, protoc)
	}
	return path, nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of vprotogen:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if !filepath.IsAbs(*directory) {
		pwd, wdErr := os.Getwd()
		if wdErr != nil {
			fmt.Println("Can not get current working directory.")
			os.Exit(1)
		}
		*directory = filepath.Join(pwd, *directory)
	}

	pwd := *directory
	GOBIN := common.GetGOBIN()
	binPath := os.Getenv("PATH")
	pathSlice := []string{pwd, GOBIN, binPath}
	binPath = strings.Join(pathSlice, string(os.PathListSeparator))
	os.Setenv("PATH", binPath)

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}

	protoc, err := whichProtoc(suffix)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	protoFilesMap := make(map[string][]string)
	walkErr := filepath.Walk(pwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Println(err)
			return err
		}

		if info.IsDir() {
			return nil
		}

		dir := filepath.Dir(path)
		filename := filepath.Base(path)
		if strings.HasSuffix(filename, ".proto") {
			path = path[len(pwd)+1:]
			protoFilesMap[dir] = append(protoFilesMap[dir], path)
		}

		return nil
	})
	if walkErr != nil {
		fmt.Println(walkErr)
		os.Exit(1)
	}

	for _, files := range protoFilesMap {
		for _, relProtoFile := range files {
			args := []string{
				"--go_out", pwd,
				"--go_opt", "paths=source_relative",
				"--go-grpc_out", pwd,
				"--go-grpc_opt", "paths=source_relative",
				"--plugin", "protoc-gen-go=" + filepath.Join(GOBIN, "protoc-gen-go"+suffix),
				"--plugin", "protoc-gen-go-grpc=" + filepath.Join(GOBIN, "protoc-gen-go-grpc"+suffix),
			}
			args = append(args, relProtoFile)
			cmd := exec.Command(protoc, args...)
			cmd.Env = append(cmd.Env, os.Environ()...)
			cmd.Dir = pwd
			output, cmdErr := cmd.CombinedOutput()
			if len(output) > 0 {
				fmt.Println(string(output))
			}
			if cmdErr != nil {
				fmt.Println(cmdErr)
				os.Exit(1)
			}
		}
	}
}
